// Package languages provides language rules to use with the inflect package.
package languages

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/inflect/types"
)

// Defines irregular words, uncountables words, and pluralization/singularization rules for the English language.
//
// FIXME: Singular/Plural rules could be better, I went to school for engineering, not English.
var English = types.Language("en").
	// Pluralization rules.
	Plural(`(auto)$`, `${1}s`).
	Plural(`(s|ss|sh|ch|x|to|ro|ho|jo)$`, `${1}es`).
	Plural(`(i)fe$`, `${1}ves`).
	Plural(`(t|f|g)oo(th|se|t)$`, `${1}ee${2}`).
	Plural(`(a|e|i|o|u)y$`, `${1}ys`).
	Plural(`(m|l)ouse$`, `${1}ice`).
	Plural(`(al|ie|l)f$`, `${1}ves`).
	Plural(`(d)ice`, `${1}ie`).
	Plural(`y$`, `ies`).
	Plural(`$`, `s`).
	// Singularization rules.
	Singular(`(auto)s$`, `${1}`).
	Singular(`(rse)s$`, `${1}`).
	Singular(`(s|ss|sh|ch|x|to|ro|ho|jo)es$`, `${1}`).
	Singular(`(i)ves$`, `${1}fe`).
	Singular(`(t|f|g)ee(th|se|t)$`, `${1}oo${2}`).
	Singular(`(a|e|i|o|u)ys$`, `${1}y`).
	Singular(`(m|l)ice$`, `${1}ouse`).
	Singular(`(al|ie|l)ves$`, `${1}f`).
	Singular(`(l)ies`, `${1}ie`).
	Singular(`ies$`, `y`).
	Singular(`(d)ie`, `${1}ice`).
	Singular(`s$`, ``).
	// Irregulars words.
	Irregular(`person`, `people`).
	Irregular(`child`, `children`).
	// Uncountables words.
	Uncountable(`fish`).
	Uncountable(`sheep`).
	Uncountable(`deer`).
	Uncountable(`tuna`).
	Uncountable(`salmon`).
	Uncountable(`trout`).
	Uncountable(`music`).
	Uncountable(`art`).
	Uncountable(`love`).
	Uncountable(`happiness`).
	Uncountable(`advice`).
	Uncountable(`information`).
	Uncountable(`news`).
	Uncountable(`furniture`).
	Uncountable(`luggage`).
	Uncountable(`rice`).
	Uncountable(`sugar`).
	Uncountable(`butter`).
	Uncountable(`water`).
	Uncountable(`electricity`).
	Uncountable(`gas`).
	Uncountable(`power`).
	Uncountable(`money`).
	Uncountable(`currency`).
	Uncountable(`scenery`)
