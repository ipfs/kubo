// Implement tests for the `ignore` library
package ignore

import (
    "os"

    "io/ioutil"
    "path/filepath"

    "fmt"
    "testing"

    "github.com/stretchr/testify/assert"
)

const (
    TEST_DIR = "test_fixtures"
)

// Helper function to setup a test fixture dir and write to
// it a file with the name "fname" and content "content"
func writeFileToTestDir(fname, content string) {
    testDirPath := "." + string(filepath.Separator) + TEST_DIR
    testFilePath := testDirPath + string(filepath.Separator) + fname

    _ = os.MkdirAll(testDirPath, 0755)
    _ = ioutil.WriteFile(testFilePath, []byte(content), os.ModePerm)
}

func cleanupTestDir() {
    _ = os.RemoveAll(fmt.Sprintf(".%s%s", string(filepath.Separator), TEST_DIR))
}

// Validate "CompileIgnoreLines()"
func TestCompileIgnoreLines(test *testing.T) {
    lines := []string{"abc/def", "a/b/c", "b"}
    object, error := CompileIgnoreLines(lines...)
    assert.Nil(test, error, "error from CompileIgnoreLines should be nil")

    // MatchesPath
    // Paths which are targeted by the above "lines"
    assert.Equal(test, true,  object.MatchesPath("abc/def/child"), "abc/def/child should match")
    assert.Equal(test, true,  object.MatchesPath("a/b/c/d"),       "a/b/c/d should match")

    // Paths which are not targeted by the above "lines"
    assert.Equal(test, false, object.MatchesPath("abc"), "abc should not match")
    assert.Equal(test, false, object.MatchesPath("def"), "def should not match")
    assert.Equal(test, false, object.MatchesPath("bd"),  "bd should not match")

    object, error = CompileIgnoreLines("abc/def", "a/b/c", "b")
    assert.Nil(test, error, "error from CompileIgnoreLines should be nil")

    // Paths which are targeted by the above "lines"
    assert.Equal(test, true,  object.MatchesPath("abc/def/child"), "abc/def/child should match")
    assert.Equal(test, true,  object.MatchesPath("a/b/c/d"),       "a/b/c/d should match")

    // Paths which are not targeted by the above "lines"
    assert.Equal(test, false, object.MatchesPath("abc"), "abc should not match")
    assert.Equal(test, false, object.MatchesPath("def"), "def should not match")
    assert.Equal(test, false, object.MatchesPath("bd"),  "bd should not match")
}

// Validate the invalid files
func TestCompileIgnoreFile_InvalidFile(test *testing.T) {
    object, error := CompileIgnoreFile("./test_fixtures/invalid.file")
    assert.Nil(test, object, "object should be nil")
    assert.NotNil(test, error, "error should be unknown file / dir")
}

// Validate the an empty files
func TestCompileIgnoreLines_EmptyFile(test *testing.T) {
    writeFileToTestDir("test.gitignore", ``)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, false, object.MatchesPath("a"),       "should not match any path")
    assert.Equal(test, false, object.MatchesPath("a/b"),     "should not match any path")
    assert.Equal(test, false, object.MatchesPath(".foobar"), "should not match any path")
}

// Validate the correct handling of the negation operator "!"
func TestCompileIgnoreLines_HandleIncludePattern(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
# exclude everything except directory foo/bar
/*
!/foo
/foo/*
!/foo/bar
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("a"),        "a should match")
    assert.Equal(test, true,  object.MatchesPath("foo/baz"),  "foo/baz should match")
    assert.Equal(test, false, object.MatchesPath("foo"),      "foo should not match")
    assert.Equal(test, false, object.MatchesPath("/foo/bar"), "/foo/bar should not match")
}

// Validate the correct handling of comments and empty lines
func TestCompileIgnoreLines_HandleSpaces(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
#
# A comment

# Another comment


    # Invalid Comment

abc/def
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, 2, len(object.patterns), "should have two regex pattern")
    assert.Equal(test, false, object.MatchesPath("abc/abc"), "/abc/abc should not match")
    assert.Equal(test, true,  object.MatchesPath("abc/def"), "/abc/def should match")
}

// Validate the correct handling of leading / chars
func TestCompileIgnoreLines_HandleLeadingSlash(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
/a/b/c
d/e/f
/g
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, 3, len(object.patterns), "should have 3 regex patterns")
    assert.Equal(test, true,  object.MatchesPath("a/b/c"),   "a/b/c should match")
    assert.Equal(test, true,  object.MatchesPath("a/b/c/d"), "a/b/c/d should match")
    assert.Equal(test, true,  object.MatchesPath("d/e/f"),   "d/e/f should match")
    assert.Equal(test, true,  object.MatchesPath("g"),       "g should match")
}

// Validate the correct handling of files starting with # or !
func TestCompileIgnoreLines_HandleLeadingSpecialChars(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
# Comment
\#file.txt
\!file.txt
file.txt
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("#file.txt"),   "#file.txt should match")
    assert.Equal(test, true,  object.MatchesPath("!file.txt"),   "!file.txt should match")
    assert.Equal(test, true,  object.MatchesPath("a/!file.txt"), "a/!file.txt should match")
    assert.Equal(test, true,  object.MatchesPath("file.txt"),    "file.txt should match")
    assert.Equal(test, true,  object.MatchesPath("a/file.txt"),  "a/file.txt should match")
    assert.Equal(test, false, object.MatchesPath("file2.txt"),   "file2.txt should not match")

}

// Validate the correct handling matching files only within a given folder
func TestCompileIgnoreLines_HandleAllFilesInDir(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
Documentation/*.html
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("Documentation/git.html"),             "Documentation/git.html should match")
    assert.Equal(test, false, object.MatchesPath("Documentation/ppc/ppc.html"),         "Documentation/ppc/ppc.html should not match")
    assert.Equal(test, false, object.MatchesPath("tools/perf/Documentation/perf.html"), "tools/perf/Documentation/perf.html should not match")
}

// Validate the correct handling of "**"
func TestCompileIgnoreLines_HandleDoubleStar(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
**/foo
bar
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("foo"),     "foo should match")
    assert.Equal(test, true,  object.MatchesPath("baz/foo"), "baz/foo should match")
    assert.Equal(test, true,  object.MatchesPath("bar"),     "bar should match")
    assert.Equal(test, true,  object.MatchesPath("baz/bar"), "baz/bar should match")
}

// Validate the correct handling of leading slash
func TestCompileIgnoreLines_HandleLeadingSlashPath(test *testing.T) {
    writeFileToTestDir("test.gitignore", `
/*.c
`)
    defer cleanupTestDir()

    object, error := CompileIgnoreFile("./test_fixtures/test.gitignore")
    assert.Nil(test, error, "error should be nil")
    assert.NotNil(test, object, "object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("hello.c"),     "hello.c should match")
    assert.Equal(test, false, object.MatchesPath("foo/hello.c"), "foo/hello.c should not match")
}

func ExampleCompileIgnoreLines() {
    ignoreObject, error := CompileIgnoreLines([]string{"node_modules", "*.out", "foo/*.c"}...)
    if error != nil {
        panic("Error when compiling ignore lines: " + error.Error())
    }

    // You can test the ignoreObject against various paths using the
    // "MatchesPath()" interface method. This pretty much is up to
    // the users interpretation. In the case of a ".gitignore" file,
    // a "match" would indicate that a given path would be ignored.
    fmt.Println(ignoreObject.MatchesPath("node_modules/test/foo.js"))
    fmt.Println(ignoreObject.MatchesPath("node_modules2/test.out"))
    fmt.Println(ignoreObject.MatchesPath("test/foo.js"))

    // Output:
    // true
    // true
    // false
}

func TestCompileIgnoreLines_CheckNestedDotFiles(test *testing.T) {
    lines := []string{
        "**/external/**/*.md",
        "**/external/**/*.json",
        "**/external/**/*.gzip",
        "**/external/**/.*ignore",

        "**/external/foobar/*.css",
        "**/external/barfoo/less",
        "**/external/barfoo/scss",
    }
    object, error := CompileIgnoreLines(lines...)
    assert.Nil(test, error, "error from CompileIgnoreLines should be nil")
    assert.NotNil(test, object, "returned object should not be nil")

    assert.Equal(test, true,  object.MatchesPath("external/foobar/angular.foo.css"), "external/foobar/angular.foo.css")
    assert.Equal(test, true,  object.MatchesPath("external/barfoo/.gitignore"), "external/barfoo/.gitignore")
    assert.Equal(test, true,  object.MatchesPath("external/barfoo/.bower.json"), "external/barfoo/.bower.json")
}
