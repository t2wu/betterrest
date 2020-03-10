package security

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// https://gist.github.com/chadlung/c617e045750b73f6fe7f2f70d99fb321
// https://stackoverflow.com/questions/44816003/jwt-key-is-invalid
// https://www.sohamkamani.com/blog/golang/2019-01-01-jwt-authentication/

const (
	// For simplicity these files are in the same folder as the app binary.
	// You shouldn't do this in production.

	// Generate key using
	// https://byparker.com/blog/2019/generating-a-golang-compatible-ssh-rsa-key-pair/
	// openssl genrsa -des3 -out private.pem 4096
	// openssl rsa -in private.pem -outform PEM -pubout -out public.pem
	// and that's it, not the third command
	// (Somedays I gotta figure out what this is all about. And ssh-keygen too.)

	privKeyPath = "private.pem"
	pubKeyPath  = "public.pem"
)

var (
	verifyKey *rsa.PublicKey
	signKey   *rsa.PrivateKey
)

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// // Token for JWT
// type Token struct {
// 	Token string `json:"token"`
// }

// init loads the public and private keys fromfile
func init() {
	signBytes, err := ioutil.ReadFile(privKeyPath)
	fatal(err)

	// If I use the API that doesn't have the password option it will say
	// asn1: structure error: tags don't match (16 vs {class:0 tag:20 length:105 isCompound:true}) ... blah
	signKey, err = jwt.ParseRSAPrivateKeyFromPEMWithPassword(signBytes, "123456")
	fatal(err)

	verifyBytes, err := ioutil.ReadFile(pubKeyPath)
	fatal(err)

	verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	fatal(err)
}

// CreateJWTToken creates a new JWT token
func CreateJWTToken(ident uint) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * time.Duration(3)).Unix(),
		"iat": time.Now().Unix(),
		"iss": ident, // The subject of the token. Unique identifier of the resource owner
	})

	tokenString, err := token.SignedString(signKey)
	return tokenString, err
}

// GetISSIfTokenIsValid validates this token and get the ISS claim
func GetISSIfTokenIsValid(tokenString string) (uint, error) {
	if tokenString == "" {
		return 0, errors.New("Empty token")
	}
	// sample token string taken from the New example
	// tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.u1riaD1rW97opCoAuRCTy4w58Br-Zk-bh7vLiRIsrpU"

	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return verifyKey, nil // returns the public key
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if ident, ok := claims["iss"]; ok {
			if ident, ok := ident.(float64); ok {
				return uint(ident), nil
			}
			return 0, errors.New("getting ISS from token error")
		}
	}

	// No ISS? Consider it invalid
	return 0, errors.New("Token has no iss field")

	// https://www.sohamkamani.com/blog/golang/2019-01-01-jwt-authentication/
	// Parse the JWT string and store the result in `claims`.
	// Note that we are passing the key in this method as well. This method will return an error
	// if the token is invalid (if it has expired according to the expiry time we set on sign in),
	// or if the signature does not match
	// token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
	// 	return verifyKey, nil
	// })
}
