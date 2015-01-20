// Package types contains common types useful to the inflect package.
package types

// LanguageType provides a structure for storing inflections rules of a language.
type LanguageType struct {
	Short            string           // The short hand form represention the language, ex. `en` (English).
	Pluralizations   RulesType        // Rules for pluralizing standard words.
	Singularizations RulesType        // Rules for singularizing standard words.
	Irregulars       IrregularsType   // Slice containing irregular words that do not follow standard rules.
	Uncountables     UncountablesType // Words that are uncountable, having the same form for both singular and plural.
}

func convert(str, form string, language *LanguageType, rules RulesType) string {
	if language.Uncountables.Contains(str) {
		return str
	} else if irregular, ok := language.Irregulars.IsIrregular(str); ok {
		if form == "singular" {
			return irregular.Singular
		}
		return irregular.Plural
	} else {
		for _, rule := range rules {
			if rule.Regexp.MatchString(str) {
				return rule.Regexp.ReplaceAllString(str, rule.Replacer)
			}
		}
	}

	return str
}

// Pluralize converts the given string to the languages plural form.
func (self *LanguageType) Pluralize(str string) string {
	return convert(str, "plural", self, self.Pluralizations)
}

// Singularize converts the given string to the languages singular form.
func (self *LanguageType) Singularize(str string) string {
	return convert(str, "singular", self, self.Singularizations)
}

// Plural defines a pluralization rule for a language.
func (self *LanguageType) Plural(matcher, replacer string) *LanguageType {
	self.Pluralizations = append(self.Pluralizations, Rule(matcher, replacer))

	return self
}

// Plural defines a singularization rule for a language.
func (self *LanguageType) Singular(matcher, replacer string) *LanguageType {
	self.Singularizations = append(self.Singularizations, Rule(matcher, replacer))

	return self
}

// Plural defines an irregular word for a langauge.
func (self *LanguageType) Irregular(singular, plural string) *LanguageType {
	self.Irregulars = append(self.Irregulars, Irregular(singular, plural))

	return self
}

// Plural defines an uncountable word for a langauge.
func (self *LanguageType) Uncountable(uncountable string) *LanguageType {
	self.Uncountables = append(self.Uncountables, uncountable)

	return self
}

// Language if a factory method to a new LanguageType.
func Language(short string) (language *LanguageType) {
	language = new(LanguageType)

	language.Pluralizations = make(RulesType, 0)
	language.Singularizations = make(RulesType, 0)
	language.Irregulars = make(IrregularsType, 0)
	language.Uncountables = make(UncountablesType, 0)

	return
}
