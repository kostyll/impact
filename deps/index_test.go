package deps

import "log"
import "fmt"
import "strings"
import "testing"
import "github.com/blang/semver"
import "github.com/stretchr/testify/assert"

/*
 * Helper function to turn a string (in the form 'LibName:Version' into a
 * library name and semantic version.
 */
func parse(libs string) (LibraryName, *semver.Version) {
	parts := strings.Split(libs, ":")
	if len(parts) != 2 {
		panic(fmt.Errorf("Invalid library spec: %s", libs))
	}
	v, err := semver.New(parts[1])
	if err != nil {
		panic(err)
	}
	return LibraryName(parts[0]), v
}

/*
 * Declare a dependency and add libraries, all in one shot.
 */
func deps(index *LibraryIndex, libs string, deps ...string) error {
	lib, libver := parse(libs)
	index.AddLibrary(lib, libver)
	for _, ds := range deps {
		dep, depver := parse(ds)
		if !index.Contains(dep, depver) {
			index.AddLibrary(dep, depver)
		}
		err := index.AddDependency(lib, libver, dep, depver)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
 * Test whether the resolution matches expected values.
 */
func testConfig(t *testing.T, config Configuration, vers ...string) {
	for _, v := range vers {
		lib, libver := parse(v)
		cver, exists := config[lib]
		assert.True(t, exists)
		if exists {
			assert.Equal(t, 0, cver.Compare(libver))
		}
	}
}

/* Simple Case: Root 1.0.0 depends on A 1.0.0 */
func TestResolution3(t *testing.T) {
	index := MakeLibraryIndex()
	err := deps(&index, "Root:1.0.0", "A:1.0.0")
	assert.NoError(t, err)
	config, err := index.Resolve("Root")
	assert.NoError(t, err)

	testConfig(t, config, "Root:1.0.0")
}

/* Circular Case:
 *   Root 1.0.0 -> A 1.0.0 AND
 *   A 1.0.0    -> Root 1.0.0
 */
func TestResolutionOfCircularDependency(t *testing.T) {
	index := MakeLibraryIndex()

	err := deps(&index, "Root:1.0.0", "A:1.0.0")
	assert.NoError(t, err)

	err = deps(&index, "A:1.0.0", "Root:1.0.0")
	assert.NoError(t, err)

	config, err := index.Resolve("Root")
	assert.NoError(t, err)

	testConfig(t, config, "Root:1.0.0", "A:1.0.0")
}

/* Unmet Circular
 *   Root 1.0.0 -> A 1.0.0 AND
 *   A 1.0.0    -> Root 1.0.1
 *
 *   Root 1.0.1 -> A 1.0.1 AND
 *   A 1.0.1    -> Root 1.0.0
 */
func TestResolutionOfUnmetCircularDependency(t *testing.T) {
	index := MakeLibraryIndex()

	err := deps(&index, "Root:1.0.0", "A:1.0.0")
	assert.NoError(t, err)

	err = deps(&index, "A:1.0.0", "Root:1.0.1")
	assert.NoError(t, err)

	err = deps(&index, "Root:1.0.1", "A:1.0.1")
	assert.NoError(t, err)

	err = deps(&index, "A:1.0.1", "Root:1.0.0")
	assert.NoError(t, err)

	/* Should yield an error, since no configuration works */
	_, err = index.Resolve("Root")
	assert.Error(t, err)
}

/*
 * This case tests the lower level (pedantic) API.
 */
func TestResolution1(t *testing.T) {
	index := MakeLibraryIndex()
	root1, err := semver.New("1.0.0")
	assert.NoError(t, err, "Parsing root1 version")
	a1, err := semver.New("1.0.0")
	assert.NoError(t, err, "Parsing a1 version")
	index.AddLibrary("Root", root1)
	err = index.AddDependency("Root", root1, "A", a1)
	assert.Error(t, err, "Should fail because A is unknown")
	index.AddLibrary("A", a1)
	err = index.AddDependency("Root", root1, "A", a1)
	assert.NoError(t, err, "Couldn't add dependency")

	rootVers := index.Versions("Root")
	assert.Equal(t, *rootVers, VersionList{root1})

	config, err := index.Resolve("Root")
	assert.NoError(t, err)
	assert.NotNil(t, config["Root"])

	log.Printf("Configuration: %v", config)

	// Introduce a circular dependency.  Make sure nothing breaks
	err = index.AddDependency("A", a1, "Root", root1)
	assert.NoError(t, err, "Couldn't add dependency")

	rootVers = index.Versions("Root")
	assert.True(t, rootVers.Contains(root1))
	assert.Equal(t, 1, rootVers.Len())

	config, err = index.Resolve("Root")
	assert.NoError(t, err)

	log.Printf("Configuration: %v", config)
}

/*
 * This case also tests the lower level (pedantic) API.
 */
func TestResolution2(t *testing.T) {
	index := MakeLibraryIndex()
	root1, err := semver.New("1.0.0")
	assert.NoError(t, err, "Parsing root1 version")
	a1, err := semver.New("1.0.0")
	assert.NoError(t, err, "Parsing a1 version")
	index.AddLibrary("Root", root1)
	index.AddLibrary("A", a1)
	err = index.AddDependency("Root", root1, "A", a1)
	assert.NoError(t, err, "Couldn't add dependency")

	rootVers := index.Versions("Root")
	assert.Equal(t, *rootVers, VersionList{root1})

	// Introduce a circular dependency that makes resolution fail
	root2, err := semver.New("1.0.1")
	a2, err := semver.New("1.0.1")
	assert.NoError(t, err, "Parsing root2 version")
	index.AddLibrary("Root", root2)
	index.AddLibrary("A", a2)
	err = index.AddDependency("Root", root2, "A", a2)
	err = index.AddDependency("A", a1, "Root", root2)
	err = index.AddDependency("A", a2, "Root", root1)
	assert.NoError(t, err, "Couldn't add dependency")

	rootVers = index.Versions("Root")
	assert.True(t, rootVers.Contains(root1))
	assert.True(t, rootVers.Contains(root2))
	assert.Equal(t, 2, rootVers.Len())

	config, err := index.Resolve("Root")
	log.Printf("Config = %v", config)
	assert.Error(t, err)

	log.Printf("Configuration: %v", config)
}