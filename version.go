package xfsgo

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

var (
	appname = "xfsgo"
	version = "0.5.11"
)

func CurrentVersion() string {
	return version
}

func VersionMajor() int {
	versionNames := strings.Split(version, ".")
	if len(versionNames) != 3 {
		return 0
	}
	num, err := strconv.ParseInt(versionNames[0], 10, 32)
	if err != nil {
		return 0
	}
	return int(num)
}

func VersionString() string {
	vs := "v" + CurrentVersion()
	osArch := runtime.GOOS + "/" + runtime.GOARCH
	return fmt.Sprintf("%s %s %s",
		appname, vs, osArch)
}

func GetAppName() string {
	return appname
}
