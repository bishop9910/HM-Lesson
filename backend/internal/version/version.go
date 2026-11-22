package version

// Version 由构建参数注入：-ldflags "-X .../internal/version.Version=0.1.0"
var Version = "0.1.0-dev"
