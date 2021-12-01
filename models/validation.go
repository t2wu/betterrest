package models

import (
	"errors"
	"log"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

// use a single instance , it caches struct info
var (
	uni      *ut.UniversalTranslator
	Validate *validator.Validate
	Trans    ut.Translator
)

func init() {
	en := en.New() // locales.Translator
	uni = ut.New(en, en)
	Trans, _ = uni.GetTranslator("en")
	Validate = validator.New()
	en_translations.RegisterDefaultTranslations(Validate, Trans)
}

func ValidateModel(modelObj IModel) error {
	err := Validate.Struct(modelObj)
	if err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			s, err2 := TranslateValidationErrorMessage(errs, modelObj)
			if err2 != nil {
				log.Println("error translating validaiton message:", err)
			}
			err = errors.New(s)
		}

		return err
	}
	return nil
}

func TranslateValidationErrorMessage(errs validator.ValidationErrors, modelObj IModel) (string, error) {
	// There could be several, outputting one for now
	for k, v := range errs.Translate(Trans) { // map[string]string
		// can have multiple, let me just do one at a time now
		toks := strings.SplitN(k, ".", 2)
		n, err := FieldNameToJSONName(modelObj, toks[1])
		if err != nil {
			return "", err
		}
		v = strings.ToLower(v)
		return n + ": " + v, nil
	}
	return "", nil // never here
}
