package common

import (
	"testing"

	"github.com/ipfs/kubo/thirdparty/assert"
)

func TestMapMergeDeepReturnsNew(t *testing.T) {
	leftMap := make(map[string]interface{})
	leftMap["A"] = "Hello World"

	rightMap := make(map[string]interface{})
	rightMap["A"] = "Foo"

	MapMergeDeep(leftMap, rightMap)

	assert.True(leftMap["A"] == "Hello World", t, "MapMergeDeep should return a new map instance")
}

func TestMapMergeDeepNewKey(t *testing.T) {
	leftMap := make(map[string]interface{})
	leftMap["A"] = "Hello World"
	/*
		leftMap
		{
			A: "Hello World"
		}
	*/

	rightMap := make(map[string]interface{})
	rightMap["B"] = "Bar"
	/*
		rightMap
		{
			B: "Bar"
		}
	*/

	result := MapMergeDeep(leftMap, rightMap)
	/*
		expected
		{
			A: "Hello World"
			B: "Bar"
		}
	*/

	assert.True(result["B"] == "Bar", t, "New keys in right map should exist in resulting map")
}

func TestMapMergeDeepRecursesOnMaps(t *testing.T) {
	leftMapA := make(map[string]interface{})
	leftMapA["B"] = "A value!"
	leftMapA["C"] = "Another value!"

	leftMap := make(map[string]interface{})
	leftMap["A"] = leftMapA
	/*
		leftMap
		{
			A: {
				B: "A value!"
				C: "Another value!"
			}
		}
	*/

	rightMapA := make(map[string]interface{})
	rightMapA["C"] = "A different value!"

	rightMap := make(map[string]interface{})
	rightMap["A"] = rightMapA
	/*
		rightMap
		{
			A: {
				C: "A different value!"
			}
		}
	*/

	result := MapMergeDeep(leftMap, rightMap)
	/*
		expected
		{
			A: {
				B: "A value!"
				C: "A different value!"
			}
		}
	*/

	resultA := result["A"].(map[string]interface{})
	assert.True(resultA["B"] == "A value!", t, "Unaltered values should not change")
	assert.True(resultA["C"] == "A different value!", t, "Nested values should be altered")
}

func TestMapMergeDeepRightNotAMap(t *testing.T) {
	leftMapA := make(map[string]interface{})
	leftMapA["B"] = "A value!"

	leftMap := make(map[string]interface{})
	leftMap["A"] = leftMapA
	/*
		origMap
		{
			A: {
				B: "A value!"
			}
		}
	*/

	rightMap := make(map[string]interface{})
	rightMap["A"] = "Not a map!"
	/*
		newMap
		{
			A: "Not a map!"
		}
	*/

	result := MapMergeDeep(leftMap, rightMap)
	/*
		expected
		{
			A: "Not a map!"
		}
	*/

	assert.True(result["A"] == "Not a map!", t, "Right values that are not a map should be set on the result")
}
