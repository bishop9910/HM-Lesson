// Package password 提供 Argon2id 密码哈希与校验。
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// 参数按 2C8G 云托管实例校准：64MB 内存、3 轮、2 并行。
// 一旦线上已有密码，修改参数必须保留旧 hash 的校验分支。
const (
	memoryKiB  = 64 * 1024
	iterations = 3
	parallel   = 2
	saltLength = 16
	keyLength  = 32
)

var errInvalidHash = errors.New("密码哈希格式无效")

// Hasher 让 password 包以接口形式注入 biz，便于测试替换。
type Hasher struct{}

func (Hasher) Hash(plain string) (string, error) { return Hash(plain) }
func (Hasher) Verify(encoded, plain string) bool { return Verify(encoded, plain) }

// Hash 返回 PHC 字符串：argon2id$v=19$m=65536,t=3,p=2$salt$hash。
func Hash(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("密码不能为空")
	}
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成盐失败: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, iterations, memoryKiB, parallel, keyLength)
	return fmt.Sprintf("argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify 以常量时间比较密码；格式不合法时返回 false。
func Verify(encoded, plain string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if m == 0 || t == 0 || p == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ParseMemory 仅用于测试/诊断，返回编码里的内存参数。
func ParseMemory(encoded string) (uint32, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return 0, errInvalidHash
	}
	values := strings.Split(parts[2], ",")
	if len(values) != 3 || !strings.HasPrefix(values[0], "m=") {
		return 0, errInvalidHash
	}
	v, err := strconv.ParseUint(strings.TrimPrefix(values[0], "m="), 10, 32)
	if err != nil {
		return 0, errInvalidHash
	}
	return uint32(v), nil
}
