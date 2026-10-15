package global

import (
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3"
)

var PULUMI_VERSION = "v" + sdk.Version.String()

// The embedded binary lock and generated-install evidence use this exact Bun.
const BUN_VERSION = "1.3.6"

const UV_VERSION = "0.3.2"

var configDir = (func() string {
	home, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	result := filepath.Join(home, "sst")
	os.MkdirAll(result, 0755)
	os.MkdirAll(filepath.Join(result, "bin"), 0755)
	return result
}())

func ConfigDir() string {
	return configDir
}

func BinPath() string {
	return filepath.Join(configDir, "bin")
}

func CertPath() string {
	return filepath.Join(configDir, "cert")
}
