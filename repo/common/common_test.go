package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapMergeDeepReturnsNew(t *testing.T) {
	leftMap := make(map[string]interface{})
	leftMap["A"] = "Hello World"

	rightMap := make(map[string]interface{})
	rightMap["A"] = "Foo"

	MapMergeDeep(leftMap, rightMap)

	require.Equal(t, "Hello World", leftMap["A"], "MapMergeDeep should return a new map instance")
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

	require.Equal(t, "Bar", result["B"], "New keys in right map should exist in resulting map")
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
	require.Equal(t, "A value!", resultA["B"], "Unaltered values should not change")
	require.Equal(t, "A different value!", resultA["C"], "Nested values should be altered")
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

	require.Equal(t, "Not a map!", result["A"], "Right values that are not a map should be set on the result")
}
