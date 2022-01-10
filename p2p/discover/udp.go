package discover

import (
	"bytes"
	"container/list"
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/crypto"
	"xfsgo/log"
	"xfsgo/p2p/nat"
)

const Version = 1

// Errors
var (
	errExpired          = errors.New("expired")
	errBadVersion       = errors.New("version mismatch")
	errUnsolicitedReply = errors.New("unsolicited reply")
	errUnknownNode      = errors.New("unknown node")
	errTimeout          = errors.New("RPC timeout")
	errClockWarp        = errors.New("reply deadline too far in the future")
	errClosed           = errors.New("socket closed")
)

// Timeouts
const (
	respTimeout = 500 * time.Millisecond
	expiration  = 20 * time.Second

	refreshInterval = 1 * time.Hour
)

// RPC packet types
const (
	pingPacket = iota + 1 // zero is 'reserved'
	pongPacket
	findnodePacket
	neighborsPacket
)

func makeEndpoint(addr *net.UDPAddr, tcpPort uint16) rpcEndpoint {
	ip := addr.IP.To4()
	if ip == nil {
		ip = addr.IP.To16()
	}
	return rpcEndpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

func nodeFromRPC(rn rpcNode) (n *Node, valid bool) {
	// TODO: don't accept localhost, LAN addresses from internet hosts
	// TODO: check public key is on secp256k1 curve
	if rn.IP.IsMulticast() || rn.IP.IsUnspecified() || rn.UDP == 0 {
		return nil, false
	}
	return newNode(rn.IP, rn.TCP, rn.UDP, rn.ID), true
}

func nodeToRPC(n *Node) rpcNode {
	return rpcNode{ID: n.ID, IP: n.IP, UDP: n.UDP, TCP: n.TCP}
}

type packet interface {
	handle(t *udp, from *net.UDPAddr, fromID NodeId) error
}

type conn interface {
	ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error)
	WriteToUDP(b []byte, addr *net.UDPAddr) (n int, err error)
	Close() error
	LocalAddr() net.Addr
}

// udp implements the RPC protocol.
type udp struct {
	//logger log.Logger
	conn        conn
	priv        *ecdsa.PrivateKey
	ourEndpoint rpcEndpoint

	addpending chan *pending
	gotreply   chan reply

	closing chan struct{}
	log     log.Logger
	*Table
}

// pending represents a pending reply.
//
// some implementations of the protocol wish to send more than one
// reply packet to findnode. in general, any neighbors packet cannot
// be matched up with a specific findnode packet.
//
// our implementation handles this by storing a callback function for
// each pending reply. incoming packets from a node are dispatched
// to all the callback functions for that node.
type pending struct {
	// these fields must match in the reply.
	from  NodeId
	ptype byte

	// time when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives. if it returns
	// true, the callback is removed from the pending reply queue.
	// if it returns false, the reply is considered incomplete and
	// the callback will be invoked again for the next matching reply.
	callback func(resp interface{}) (done bool)

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout.
	errc chan<- error
}

type reply struct {
	from  NodeId
	ptype byte
	data  interface{}
	// loop indicates whether there was
	// a matching request by sending on this channel.
	matched chan<- bool
}

// ListenUDP returns a new table that listens for UDP packets on laddr.
func ListenUDP(priv *ecdsa.PrivateKey, laddr string, nodeDBPath string, mapper nat.Mapper, log log.Logger) (*Table, error) {
	addr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	tab, _ := newUDP(priv, conn, nodeDBPath, mapper, log)
	return tab, nil
}
func NewUDP(priv *ecdsa.PrivateKey, c conn, nodeDBPath string, mapper nat.Mapper, log log.Logger) (*Table, *udp) {
	return newUDP(priv, c, nodeDBPath, mapper, log)
}
func newUDP(priv *ecdsa.PrivateKey, c conn, nodeDBPath string, mapper nat.Mapper, log log.Logger) (*Table, *udp) {
	udp := &udp{
		log:        log,
		conn:       c,
		priv:       priv,
		closing:    make(chan struct{}),
		gotreply:   make(chan reply),
		addpending: make(chan *pending),
	}
	realaddr := c.LocalAddr().(*net.UDPAddr)
	if mapper != nil && !realaddr.IP.IsLoopback() {
		go nat.Map(mapper, udp.closing, "udp", realaddr.Port, realaddr.Port, "xlibp2p discovery")
	} else if mapper != nil {
		if ext, err := mapper.ExternalIP(); err == nil {
			realaddr = &net.UDPAddr{IP: ext, Port: realaddr.Port}
		}
	}
	udp.ourEndpoint = makeEndpoint(realaddr, uint16(realaddr.Port))
	udp.Table = newTable(udp, PubKey2NodeId(priv.PublicKey), realaddr, nodeDBPath, log)
	go udp.loop()
	go udp.readLoop()
	return udp.Table, udp
}

func (t *udp) close() {
	close(t.closing)
	_ = t.conn.Close()
	// TODO: wait for the loops to end.
}

// ping sends a ping message to the given node and waits for a reply.
func (t *udp) ping(toid NodeId, toaddr *net.UDPAddr) error {
	// TODO: maybe check for ReplyTo field in callback to measure RTT
	errc := t.pending(toid, pongPacket, func(interface{}) bool { return true })
	_ = t.sendN(toaddr, pingPacket, ping{
		Version:    Version,
		From:       t.ourEndpoint,
		To:         makeEndpoint(toaddr, 0), // TODO: maybe use known TCP port from DB
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}, toid)
	return <-errc
}

func (t *udp) waitping(from NodeId) error {
	return <-t.pending(from, pingPacket, func(interface{}) bool { return true })
}

// findnode sends a findnode request to the given node and waits until
// the node has sent up to k neighbors.
func (t *udp) findnode(toid NodeId, toaddr *net.UDPAddr, target NodeId) ([]*Node, error) {
	nodes := make([]*Node, 0, bucketSize)
	nreceived := 0
	errc := t.pending(toid, neighborsPacket, func(r interface{}) bool {
		reply := r.(*neighbors)
		for _, rn := range reply.Nodes {
			nreceived++
			if n, valid := nodeFromRPC(rn); valid {
				nodes = append(nodes, n)
			}
		}
		return nreceived >= bucketSize
	})
	_ = t.sendN(toaddr, findnodePacket, findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}, toid)
	err := <-errc
	return nodes, err
}

// pending adds a reply callback to the pending reply queue.
// see the documentation of type pending for a detailed explanation.
func (t *udp) pending(id NodeId, ptype byte, callback func(interface{}) bool) <-chan error {
	ch := make(chan error, 1)
	p := &pending{from: id, ptype: ptype, callback: callback, errc: ch}
	select {
	case t.addpending <- p:
		// loop will handle it
	case <-t.closing:
		ch <- errClosed
	}
	return ch
}

func (t *udp) handleReply(from NodeId, ptype byte, req packet) bool {
	matched := make(chan bool)
	select {
	case t.gotreply <- reply{from, ptype, req, matched}:
		// loop will handle it
		return <-matched
	case <-t.closing:
		return false
	}
}

// loop runs in its own goroutin. it keeps track of
// the refresh timer and the pending reply queue.
func (t *udp) loop() {
	var (
		plist       = list.New()
		timeout     = time.NewTimer(0)
		nextTimeout *pending // head of plist when timeout was last reset
		refresh     = time.NewTicker(refreshInterval)
	)
	<-timeout.C // ignore first timeout
	defer refresh.Stop()
	defer timeout.Stop()

	resetTimeout := func() {
		if plist.Front() == nil || nextTimeout == plist.Front().Value {
			return
		}
		// Start the timer so it fires when the next pending reply has expired.
		now := time.Now()
		for el := plist.Front(); el != nil; el = el.Next() {
			nextTimeout = el.Value.(*pending)
			//t.logger.Debugf("udp loop resetTimeout from: %s, packet type: %x, now: %s, next timeout: %s", nextTimeout.from, nextTimeout.ptype, now.Format(time.RFC3339), nextTimeout.deadline.Format(time.RFC3339))
			if dist := nextTimeout.deadline.Sub(now); dist < 2*respTimeout {
				//t.logger.Debugf("udp loop from: %s, packet type: %x, resetTimeout: %s",nextTimeout.from,nextTimeout.ptype, dist)
				timeout.Reset(dist)
				return
			}
			// Remove pending replies whose deadline is too far in the
			// future. These can occur if the system clock jumped
			// backwards after the deadline was assigned.
			nextTimeout.errc <- errClockWarp
			plist.Remove(el)
		}
		nextTimeout = nil
		timeout.Stop()
	}

	for {
		resetTimeout()

		select {
		case <-refresh.C:
			go t.refresh()

		case <-t.closing:
			for el := plist.Front(); el != nil; el = el.Next() {
				el.Value.(*pending).errc <- errClosed
			}
			return

		case p := <-t.addpending:
			p.deadline = time.Now().Add(respTimeout)

			//t.logger.Debugf("udp loop receive addpending chan from: %s, type: %x, deadline: %s",p.from,p.ptype, p.deadline.Format(time.RFC3339))
			plist.PushBack(p)

		case r := <-t.gotreply:
			//t.logger.Debugf("udp loop receive gotreply chan from: %s, type: %x", r.from, r.ptype)
			var matched bool
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if p.from == r.from && p.ptype == r.ptype {
					matched = true
					// Remove the matcher if its callback indicates
					// that all replies have been received. This is
					// required for packet types that expect multiple
					// reply packets.
					if p.callback(r.data) {
						p.errc <- nil
						plist.Remove(el)
					}
				}
			}
			r.matched <- matched

		case now := <-timeout.C:
			nextTimeout = nil
			// Notify and remove callbacks whose deadline is in the past.
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				//t.logger.Debugf("udp loop timeout tick, now: %s, p.deadline: %s", now, p.deadline)
				if now.After(p.deadline) || now.Equal(p.deadline) {
					p.errc <- errTimeout
					plist.Remove(el)
				}
			}
		}
	}
}
func (t *udp) sendN(toaddr *net.UDPAddr, ptype byte, req interface{}, to NodeId) error {
	packet, err := encodePacketN(t.priv, ptype, req, to)
	if err != nil {
		return err
	}
	//t.logger.Infof(">>> %v %T\n", toaddr, req)
	if _, err = t.conn.WriteToUDP(packet, toaddr); err != nil {
		//t.logger.Errorln("UDP send failed:", err)
	}
	return err
}
func (t *udp) send(toaddr *net.UDPAddr, ptype byte, req interface{}) error {
	packet, err := encodePacket(t.priv, ptype, req)
	if err != nil {
		return err
	}
	//t.logger.Infof(">>> %v %T\n", toaddr, req)
	if _, err = t.conn.WriteToUDP(packet, toaddr); err != nil {
		//t.logger.Errorln("UDP send failed:", err)
	}
	return err
}

func encodePacket(privateKey *ecdsa.PrivateKey, ptype byte, req interface{}) ([]byte, error) {
	b := new(bytes.Buffer)
	b.WriteByte(ptype)
	//nId := PubKey2NodeId(privateKey.PublicKey)
	//b.Write(nId[:])
	bs, err := rawencode.Encode(req)
	if err != nil {
		return nil, err
	}
	datahash := ahash.SHA256(bs)
	signed, err := crypto.ECDSASign(datahash, privateKey)
	if err != nil {
		return nil, err
	}

	b.Write(signed)
	// bsLen := uint32(len(bs))
	// if (bsLen >> 8) > 0 {
	// 	return nil, fmt.Errorf("out")
	// }
	b.WriteByte(byte(uint32(len(bs))))
	b.Write(bs)
	return b.Bytes(), nil
}

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	_ = binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)

	var x int32
	_ = binary.Read(bytesBuffer, binary.BigEndian, &x)

	return int(x)
}

func encodePacketN(privateKey *ecdsa.PrivateKey, ptype byte, req interface{}, remote NodeId) ([]byte, error) {
	b := new(bytes.Buffer)
	b.WriteByte(ptype)
	//nId := PubKey2NodeId(privateKey.PublicKey)
	//b.Write(nId[:])
	bs, err := rawencode.Encode(req)
	if err != nil {
		return nil, err
	}
	datahash := ahash.SHA256(bs)
	signed, err := crypto.ECDSASign(datahash, privateKey)
	if err != nil {
		return nil, err
	}
	b.Write(signed)
	b.Write(remote[:])
	dataLen := len(bs)
	// if (dataLen >> 8) > 0 {
	// 	return nil, fmt.Errorf("out")
	// }
	b.Write(IntToBytes(dataLen))
	b.Write(bs)
	return b.Bytes(), nil
}

// readLoop runs in its own goroutine. it handles incoming UDP packets.
func (t *udp) readLoop() {
	defer func() {
		if err := t.conn.Close(); err != nil {
			//t.logger.Errorln(err)
		}
	}()
	// Discovery packets are defined to be no larger than 1280 bytes.
	// Packets larger than this size will be cut at the end and treated
	// as invalid because their hash won't match.
	buf := make([]byte, 1280)
	for {
		nbytes, from, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		err = t.handlePacketN(from, buf[:nbytes])
		if err != nil {
			continue
		}
	}
}
func (t *udp) handlePacketN(from *net.UDPAddr, buf []byte) error {
	buffer := bytes.NewBuffer(buf)
	packet, fromID, err := decodePacketN(buffer, t.self.ID)
	if err != nil {
		//t.log.Debugf("Bad packet from %v: %v", from, err)
		return err
	}
	//status := "ok"
	if err = packet.handle(t, from, fromID); err != nil {
		//status = err.Error()
	}

	//t.logger.Infof("<<< %v %T: %s\n", from, packet, status)
	return err
}
func (t *udp) handlePacket(from *net.UDPAddr, buf []byte) error {
	buffer := bytes.NewBuffer(buf)
	packet, fromID, err := decodePacket(buffer)
	if err != nil {
		//t.logger.Debugf("Bad packet from %v: %v", from, err)
		return err
	}
	//status := "ok"
	if err = packet.handle(t, from, fromID); err != nil {
		//status = err.Error()
	}

	//t.logger.Infof("<<< %v %T: %s\n", from, packet, status)
	return err
}

type packetHeadN struct {
	mType   uint8
	sign    [65]byte
	dataLen uint32
	remote  NodeId
}
type packetHead struct {
	mType   uint8
	sign    [65]byte
	dataLen uint8
}

// headerLen = type(1) + sign(65) + len(1)
const headerLen = 67

func decodePacketHead(reader io.Reader) (*packetHead, int, error) {
	// 0000
	var hbytes [headerLen]byte
	last := 0
	for last < headerLen {
		var buf [headerLen]byte
		n, err := reader.Read(buf[:])
		if err != nil && err == io.EOF {
			return nil, last, err
		}
		copy(hbytes[last:last+n], buf[:n])
		last += n
	}
	ph := new(packetHead)
	mType, sign, dataLen := hbytes[0], hbytes[1:66], hbytes[66]
	ph.mType = mType
	copy(ph.sign[:], sign[:])
	ph.dataLen = dataLen
	return ph, last, nil
}

// headerLen = type(1) + sign(65) + remote(64) + len(4)
const headerLen1 = 134

func decodePacketHeadN(reader io.Reader) (*packetHeadN, int, error) {
	// 0000
	var hbytes [headerLen1]byte
	last := 0
	for last < headerLen1 {
		var buf [headerLen1]byte
		n, err := reader.Read(buf[:])
		if err != nil && err == io.EOF {
			return nil, last, err
		}
		copy(hbytes[last:last+n], buf[:n])
		last += n
	}
	ph := new(packetHeadN)
	mType, sign, remote, dataLen := hbytes[0], hbytes[1:66], hbytes[66:130], hbytes[130:134]
	ph.mType = mType
	copy(ph.sign[:], sign[:])
	copy(ph.remote[:], remote)
	ph.dataLen = uint32(BytesToInt(dataLen))
	return ph, last, nil
}

func recoverNodeId(hash, sig []byte) (NodeId, error) {
	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return NodeId{}, err
	}
	return PubKey2NodeId(*pubKey), nil
}
func decodePacketN(reader io.Reader, self NodeId) (packet, NodeId, error) {
	h, _, err := decodePacketHeadN(reader)
	if err != nil {
		return nil, NodeId{}, err
	}
	if !bytes.Equal(self[:], h.remote[:]) {
		return nil, NodeId{}, fmt.Errorf("id not match")
	}
	var data = make([]byte, h.dataLen)
	last := uint32(0)
	for last < h.dataLen {
		var buf = make([]byte, h.dataLen)
		n, err := reader.Read(buf[:])
		if err != nil && err == io.EOF {
			return nil, NodeId{}, err
		}
		copy(data[last:int(last)+n], buf[:n])
		last += uint32(n)
	}
	datahash := ahash.SHA256(data)
	nid, err := recoverNodeId(datahash, h.sign[:])
	if err != nil {
		return nil, nid, err
	}
	var req packet
	switch ptype := h.mType; ptype {
	case pingPacket:
		req = new(ping)
	case pongPacket:
		req = new(pong)
	case findnodePacket:
		req = new(findnode)
	case neighborsPacket:
		req = new(neighbors)
	default:
		return nil, nid, fmt.Errorf("unknown type: %d", ptype)
	}
	err = rawencode.Decode(data, req)
	return req, nid, err
}
func decodePacket(reader io.Reader) (packet, NodeId, error) {
	h, _, err := decodePacketHead(reader)
	if err != nil {
		return nil, NodeId{}, err
	}
	var data = make([]byte, h.dataLen)
	last := uint8(0)
	for last < h.dataLen {
		var buf [^uint8(0)]byte
		n, err := reader.Read(buf[:])
		if err != nil && err == io.EOF {
			return nil, NodeId{}, err
		}
		copy(data[last:int(last)+n], buf[:n])
		last += uint8(n)
	}
	datahash := ahash.SHA256(data)
	nid, err := recoverNodeId(datahash, h.sign[:])
	if err != nil {
		return nil, nid, err
	}
	var req packet
	switch ptype := h.mType; ptype {
	case pingPacket:
		req = new(ping)
	case pongPacket:
		req = new(pong)
	case findnodePacket:
		req = new(findnode)
	case neighborsPacket:
		req = new(neighbors)
	default:
		return nil, nid, fmt.Errorf("unknown type: %d", ptype)
	}
	err = rawencode.Decode(data, req)
	return req, nid, err
}
