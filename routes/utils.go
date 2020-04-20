package routes

package utils

import (
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// createTokenPayloadForScope creates token JSON payload
// Follow oauth's
// { acceess_token: acces_token, token_type: "Bearer", refresh_token: refresh_token, scope: ""}
func createTokenPayloadForScope(id *datatypes.UUID, scope *string) (map[string]interface{}, error) {
	var accessToken, refreshToken string
	var err error
	accessToken, err = security.CreateAccessToken(id, time.Hour*time.Duration(3), scope) // 3 hours
	if err != nil {
		return nil, err
	}

	refreshToken, err = security.CreateRefreshToken(id, time.Hour*24*time.Duration(60), scope) // 60 days
	if err != nil {
		return nil, err
	}

	retval := map[string]interface{}{
		"code": 0,
		"content": map[string]interface{}{
			"accessToken":  accessToken,
			"tokenType":    "Bearer",
			"refreshToken": refreshToken,
		},
	}

	return retval, nil
}
