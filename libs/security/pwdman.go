package security

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// https://medium.com/@jcox250/password-hash-salt-using-golang-b041dc94cb72

// HashAndSalt turns password into encrypted hash
// The salt is included in the hash
func HashAndSalt(pwd string) (string, error) {
	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	if err != nil {
		return string(hash), err
	}

	// GenerateFromPassword returns a byte slice so we need to
	// convert the bytes to a string and return it
	return string(hash), nil
}

// IsSamePassword checks if password is the same as in the db
func IsSamePassword(plainPwd string, hashedPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))

	if err != nil {
		log.Println("Error in IsSamePassword:", err)
		return false
	}

	return true
}
