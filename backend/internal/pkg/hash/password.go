package hash

import "golang.org/x/crypto/bcrypt"

// Cost 与建表脚本约定一致：bcrypt 默认 cost=10。
const Cost = 10

// Hash 生成密码哈希。
func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), Cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify 校验密码，恒定时间比较由 bcrypt 保证。
func Verify(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
