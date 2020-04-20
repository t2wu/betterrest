package security

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"

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

	accessTokenPrivKeyPath = "accesstoken-private.pem"
	accessTokenPubKeyPath  = "accesstoken-public.pem"

	refreshTokenPrivKeyPath = "refreshtoken-private.pem"
	refreshTokenPubKeyPath  = "refreshtoken-public.pem"
)

var (
	accessTokenVerifyKey *rsa.PublicKey
	accessTokenSignKey   *rsa.PrivateKey

	refreshTokenVerifyKey *rsa.PublicKey
	refreshTokenSignKey   *rsa.PrivateKey
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
	// Access Token
	signBytes, err := ioutil.ReadFile(accessTokenPrivKeyPath)
	fatal(err)

	// If I use the API that doesn't have the password option it will say
	// asn1: structure error: tags don't match (16 vs {class:0 tag:20 length:105 isCompound:true}) ... blah
	accessTokenSignKey, err = jwt.ParseRSAPrivateKeyFromPEMWithPassword(signBytes, "123456")
	fatal(err)

	verifyBytes, err := ioutil.ReadFile(accessTokenPubKeyPath)
	fatal(err)

	accessTokenVerifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	fatal(err)

	// Refresh token
	signBytes, err = ioutil.ReadFile(refreshTokenPrivKeyPath)
	fatal(err)
	refreshTokenSignKey, err = jwt.ParseRSAPrivateKeyFromPEMWithPassword(signBytes, "123456")
	fatal(err)
	verifyBytes, err = ioutil.ReadFile(refreshTokenPubKeyPath)
	fatal(err)
	refreshTokenVerifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	fatal(err)
}

// CreateAccessToken creates a new JWT token
func CreateAccessToken(ident *datatypes.UUID, duration time.Duration, scope *string) (string, error) {
	// Support both UUID and id
	claims := jwt.MapClaims{
		// "exp": time.Now().Add(time.Hour * time.Duration(3)).Unix(),
		"exp": time.Now().Add(duration).Unix(),
		"iat": time.Now().Unix(),
		"iss": ident.String(),
	}

	// if scope
	if scope != nil {
		claims["scope"] = *scope
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	tokenString, err := token.SignedString(accessTokenSignKey)
	return tokenString, err
}

// CreateRefreshToken creates a new JWT token
func CreateRefreshToken(ident *datatypes.UUID, duration time.Duration, scope *string) (string, error) {
	// Support both UUID and id
	claims := jwt.MapClaims{
		// "exp": time.Now().Add(time.Hour * 24 * time.Duration(60)).Unix(), // 60 days refresh token
		"exp": time.Now().Add(duration).Unix(), // 60 days refresh token
		"iat": time.Now().Unix(),
		"iss": ident.String(),
	}

	// if scope
	if scope != nil {
		claims["scope"] = *scope
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	tokenString, err := token.SignedString(refreshTokenSignKey)
	return tokenString, err
}

// GetClaimsIfAccessTokenIsValid validates this token and get the ISS claim
func GetClaimsIfAccessTokenIsValid(tokenString string) (*jwt.MapClaims, error) {
	if tokenString == "" {
		return nil, errors.New("Empty token")
	}
	// sample token string taken from the New example
	// tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.u1riaD1rW97opCoAuRCTy4w58Br-Zk-bh7vLiRIsrpU"

	// https://www.sohamkamani.com/blog/golang/2019-01-01-jwt-authentication/
	// Parse the JWT string and store the result in `claims`.
	// Note that we are passing the key in this method as well. This method will return an error
	// if the token is invalid (if it has expired according to the expiry time we set on sign in),
	// or if the signature does not match

	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return accessTokenVerifyKey, nil // returns the public key
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		log.Println("return claims here:", claims)
		return &claims, nil
		// if ident, ok := claims["iss"]; ok {
		// 	if ident, ok := ident.(string); ok {
		// 		return datatypes.NewUUIDFromString(ident)
		// 	}
		// 	return nil, errors.New("getting ISS from token error")
		// }
	}

	// No ISS? Consider it invalid
	return nil, errors.New("Token has no iss field")
}

// GetClaimsIfRefreshTokenIsValid validates this token and get the ISS claim
func GetClaimsIfRefreshTokenIsValid(tokenString string) (*jwt.MapClaims, error) {
	if tokenString == "" {
		return nil, errors.New("Empty token")
	}
	// sample token string taken from the New example
	// tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJuYmYiOjE0NDQ0Nzg0MDB9.u1riaD1rW97opCoAuRCTy4w58Br-Zk-bh7vLiRIsrpU"

	// https://www.sohamkamani.com/blog/golang/2019-01-01-jwt-authentication/
	// Parse the JWT string and store the result in `claims`.
	// Note that we are passing the key in this method as well. This method will return an error
	// if the token is invalid (if it has expired according to the expiry time we set on sign in),
	// or if the signature does not match

	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return refreshTokenVerifyKey, nil // returns the public key
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &claims, nil
		// if ident, ok := claims["iss"]; ok {
		// 	if ident, ok := ident.(string); ok {
		// 		return datatypes.NewUUIDFromString(ident)
		// 	}
		// 	return nil, errors.New("getting ISS from token error")
		// }
	}

	// No ISS? Consider it invalid
	return nil, errors.New("Token has no iss field")
}
