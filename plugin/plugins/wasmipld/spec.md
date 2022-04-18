---
title: "WAC Specification"
navTitle: "Spec"
---

# Specification: WAC

**Status: Prescriptive - Exploratory **

* [Format](#format)
* [Links](#links)
* [Map Keys](#map-keys)
* [Strictness](#strictness)
* [Implementations](#implementations)
    * [JavaScript](#javascript)
    * [Go](#go)
    * [Java](#java)
* [Limitations](#limitations)
    * [JavaScript Numbers](#javascript-numbers)

WAC supports bidirectional transport to and from the [IPLD Data Model].
In that way it is both a "complete" IPLD Data Model representation and "fitted" to the IPLD Data Model.
Terminology taken from [Codecs and Completeness].

It takes some inspiration from [Simple DAG], but differs in ways designed to make it bidrectional with the IPLD Data Model.

## Format

# Spec

This format is a series of typing tokens, constant tokens, and inline value data.

Typeing tokens are proceeded with the value data for that type.

Every type value can be parsed knowing only the type and without any outside context
like the container or positional delimiters.

Tokens

## TODO: Make constants in code match spec

| Int | Token |
|---|---|
| 0 | TYPE_LINK |
| 1 | TYPE_INTEGER |
| 2 | TYPE_NEGATIVE_INTEGER |
| 3 | TYPE_FLOAT |
| 4 | TYPE_NEGATIVE_FLOAT |
| 5 | TYPE_STRING |
| 6 | TYPE_BINARY |
| 7 | TYPE_MAP |
| 8 | TYPE_LIST |
| 9 | VALUE_NULL |
| 10 | VALUE_TRUE |
| 11 | VALUE_FALSE |

## TYPE_LINK

```
| 0 | CID |
```

Note: Simple-DAG put a VARINT_LENGTH before the CID indicating how long it was.

CIDs are self-delimiting so this didn't seem necessary.

## TYPE_INTEGER

```
| 1 | VARINT |
```

## TYPE_SIGNED_INTEGER

```
| 2 | VARINT |
```

## TYPE_FLOAT

```
| 3 | MATISSA_LENGTH | VARINT
```

TODO: Floats (and negative floats) need definition here.
Making implementations actually bidirectional with respect to the IPLD Data Model seems difficult here.

## TYPE_NEGATIVE_FLOAT

```
| 4 | MATISSA_LENGTH | VARINT
```

## TYPE_STRING

```
| 5 | VARINT_LENGTH | STRING
```

Note: This is essentially the same as Binary, but with a different token flag

## TYPE_BINARY

```
| 6 | VARINT_LENGTH | BINARY
```

## TYPE_MAP

```
| 7 | VARINT_NUM_PAIRS | PAIRS
```

`PAIRS` contains alternating keys then values concatenated. i.e. ` KEY1 | VALUE1 | KEY2 | VALUE2 ...`

This codec does not have any form of canonical map sorting as that would make it ill-fitted to the IPLD Data Model.

As in the IPLD Data Model map keys must be of type String, however as described in the String section the only
distinction between Strings and Bytes are identifier hints.

Note: Simple-DAG went with `KEYS_VARINT_LENGTH | VALUES_VARINT_LENGTH | KEYS | VALUES |`. Both seem doable,
this approach seemed to make writing encoder/decoders really simple. However, adding in more length prefixes
makes creating faster "zero copy" decoders very nice. It seems to mostly be a tradeoff for which side has to have bigger
buffers, the encoder or the decoder.

Note: We could assert that keys are just `VARINT_LENGTH | STRING` and remove the String token since it's always a string.
It's some added complexity and really stops us from putting anything other than Strings in map keys, but that may not be
too bad.

## TYPE_LIST

```
| 8 | VARINT_NUM_ELEMENTS | VALUES |
```

`VALUES` contains every value concatenated.

Note: Simple-DAG went with `VARINT_LENGTH` (the size of the VALUES binary section) instead of `VARINT_NUM_ELEMENTS`
and in the `VALUES` section had every value proceeded by the length of the value.

Both seem doable, this approach seemed to make writing a decoder really simple. However, adding in more length prefixes
makes creating faster "zero copy" decoders very nice. It seems to mostly be a tradeoff for which side has to have bigger
buffers, the encoder or the decoder.

## TYPE_NULL

```
| 9 |
```

## TYPE_TRUE

```
| 10 |
```

## TYPE_FALSE

```
| 11 |
```

## Strings



## Strictness



## Implementations

### JavaScript

**[@ipld/dag-cbor]**, for use with [multiformats] adheres to this specification, with the following caveats:
* Complete strictness is not yet enforced on decode. Specifically: correct map ordering is not enforced and floats that are not encoded as 64-bit are not rejected.
* [`BigInt`] is accepted along with `Number` for encode, but the smallest-possible rule is followed when encoding. When decoding integers outside of the JavaScript "safe integer" range, a [`BigInt`] will be used.

The legacy **[ipld-dag-cbor]** implementation adheres to this specification, with the following caveats:

* Strictness is not enforced on decode; blocks encoded that do not follow the strictness rules are not rejected.
* Floating point values are encoded as their smallest form rather than always 64-bit.
* Many additional object types outside of the Data Model are currently accepted for decode and encode, including `undefined`.
* [IEEE 754] special values `NaN`, `Infinity` and `-Infinity` are accepted for decode and encode.
* Integers outside of the JavaScript "safe integer" range will use the third-party [bignumber.js] library to represent their values.

Note that inability to clearly differentiate between integers and floats in JavaScript may cause problems with round-trips of floating point values. See the [IPLD Data Model] and the discussion on [Limitations](#limitations) below for further discussion on JavaScript numbers and recommendations regarding the use of floats.

### Go

Here and adheres to the specification. However, in a practical sense because it implements the go-ipld-prime interface
its limits on integers and floats are currently bounded by those interfaces. Similarly any varints are capped around 64 bits.

### Rust/WASM

Adheres to the specification. However, in a practical sense its limits on integers and floats are currently bounded to be
of fixed maximum sizes. Similarly, all varints are capped around 64 bits.

## Limitations

[IPLD Data Model]: /docs/data-model/
[Concise Binary Object Representation (CBOR)]: https://cbor.io/
[IPLD Data Model Kinds]: /docs/data-model/kinds/
[Links]: /docs/data-model/kinds/#link-kind
[CID]: /glossary/#cid
[Multibase]: https://github.com/multiformats/multibase
[go-ipld-prime]: http://github.com/ipld/go-ipld-prime
[Codecs and Completeness] : https://gist.github.com/warpfork/28f93bee7184a708223274583109f31c
[Simple DAG] : https://github.com/mikeal/simple-dag