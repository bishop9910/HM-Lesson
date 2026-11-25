package version

// Version 是当前发布版本，随每次上线递增。
// 可在构建时覆盖：-ldflags "-X .../internal/version.Version=1.3.6"
//
// 版本规则（2026-11-25 起）：
//
//	1.3.6 = 第一版第 36 次发布（1.3 是第一版主版本，末位是发布次数）
var Version = "1.3.6"
