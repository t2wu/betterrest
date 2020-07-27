package models

import (
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

// use a single instance , it caches struct info
var (
	uni      *ut.UniversalTranslator
	Validate *validator.Validate
)

func init() {
	en := en.New() // locales.Translator
	uni = ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	Validate = validator.New()
	en_translations.RegisterDefaultTranslations(Validate, trans)
}
