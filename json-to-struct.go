// gojson generates go struct defintions from JSON documents
//
// Reads from stdin and prints to stdout
//
// Example:
// 	curl -s https://api.github.com/repos/chimeracoder/gojson | gojson -name=Repository
//
// Output:
// 	package main
//
// 	type User struct {
// 		AvatarURL         string      `json:"avatar_url"`
// 		Bio               interface{} `json:"bio"`
// 		Blog              string      `json:"blog"`
// 		Company           string      `json:"company"`
// 		CreatedAt         string      `json:"created_at"`
// 		Email             string      `json:"email"`
// 		EventsURL         string      `json:"events_url"`
// 		Followers         float64     `json:"followers"`
// 		FollowersURL      string      `json:"followers_url"`
// 		Following         float64     `json:"following"`
// 		FollowingURL      string      `json:"following_url"`
// 		GistsURL          string      `json:"gists_url"`
// 		GravatarID        string      `json:"gravatar_id"`
// 		Hireable          bool        `json:"hireable"`
// 		HtmlURL           string      `json:"html_url"`
// 		ID                float64     `json:"id"`
// 		Location          string      `json:"location"`
// 		Login             string      `json:"login"`
// 		Name              string      `json:"name"`
// 		OrganizationsURL  string      `json:"organizations_url"`
// 		PublicGists       float64     `json:"public_gists"`
// 		PublicRepos       float64     `json:"public_repos"`
// 		ReceivedEventsURL string      `json:"received_events_url"`
// 		ReposURL          string      `json:"repos_url"`
// 		StarredURL        string      `json:"starred_url"`
// 		SubscriptionsURL  string      `json:"subscriptions_url"`
// 		Type              string      `json:"type"`
// 		UpdatedAt         string      `json:"updated_at"`
// 		URL               string      `json:"url"`
// 	}
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Style string

const (
	StyleUnderscores Style = "underscores"
	StyleCamelCase   Style = "camelcase"
	StyleNone        Style = "none"
)

func IsValidStyle(style Style) bool {
	return style == StyleUnderscores ||
		style == StyleCamelCase ||
		style == StyleNone
}

var (
	name            = flag.String("name", "Foo", "the name of the struct")
	pkg             = flag.String("pkg", "main", "the name of the package for the generated code")
	jsonStyleString = flag.String("json-style", string(StyleUnderscores),
		fmt.Sprintf(`the style of json struct tags: "%s", "%s", or "%s"`,
			StyleUnderscores, StyleCamelCase, StyleNone))
	jsonStyle       = StyleUnderscores
	jsonExtra       = flag.String("json-extra", "", "any extras to add to the tag")
	bsonStyleString = flag.String("bson-style", string(StyleNone),
		fmt.Sprintf(`the style of bson struct tags: "%s", "%s", or "%s"`,
			StyleUnderscores, StyleCamelCase, StyleNone))
	bsonStyle = StyleNone
	bsonExtra = flag.String("bson-extra", "", "any extras to add to the tag")
)

// Given a JSON string representation of an object and a name structName,
// attemp to generate a struct definition
func generate(input io.Reader, structName, pkgName string) ([]byte, error) {
	var iresult interface{}
	var result map[string]interface{}
	if err := json.NewDecoder(input).Decode(&iresult); err != nil {
		return nil, err
	}

	switch iresult := iresult.(type) {
	case map[string]interface{}:
		result = iresult
	case []map[string]interface{}:
		if len(iresult) > 0 {
			result = iresult[0]
		} else {
			return nil, fmt.Errorf("empty array")
		}
	default:
		return nil, fmt.Errorf("unexpected type: %T", iresult)
	}

	src := fmt.Sprintf("package %s\ntype %s %s}",
		pkgName,
		structName,
		generateTypes(result, 0))
	formatted, err := format.Source([]byte(src))
	if err != nil {
		err = fmt.Errorf("error formatting: %s, was formatting\n%s", err, src)
	}
	return formatted, err
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// Generate go struct entries for a map[string]interface{} structure
func generateTypes(obj map[string]interface{}, depth int) string {
	structure := "struct {"

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := obj[key]
		valueType := typeForValue(value)

		//If a nested value, recurse
		switch value := value.(type) {
		case []map[string]interface{}:
			valueType = "[]" + generateTypes(value[0], depth+1) + "}"
		case map[string]interface{}:
			valueType = generateTypes(value, depth+1) + "}"
		}

		fieldName := fmtFieldName(key, true)

		structure += fmt.Sprintf("\n%s %s", fieldName, valueType)

		if jsonStyle != StyleNone || bsonStyle != StyleNone {
			structure += "`"
			if bsonStyle != StyleNone {
				fieldName := ""
				if bsonStyle == StyleUnderscores {
					fieldName = key
				} else if bsonStyle == StyleCamelCase {
					fieldName = lowerFirst(fmtFieldName(key, false))
				}
				if *bsonExtra != "" {
					fieldName += "," + *bsonExtra
				}
				structure += fmt.Sprintf(`bson:"%s"`, fieldName)

				if jsonStyle != StyleNone {
					structure += " "
				}
			}
			if jsonStyle != StyleNone {
				fieldName := ""
				if jsonStyle == StyleUnderscores {
					fieldName = key
				} else if jsonStyle == StyleCamelCase {
					fieldName = lowerFirst(fmtFieldName(key, false))
				}
				if *jsonExtra != "" {
					fieldName += "," + *jsonExtra
				}
				structure += fmt.Sprintf(`json:"%s"`, fieldName)
			}
			structure += "`"
		}
	}
	return structure
}

var uppercaseFixups = map[string]bool{"id": true, "url": true, "dob": true}

// fmtFieldName formats a string as a struct key
//
// Example:
// 	fmtFieldName("foo_id", true)
// Output: FooID
func fmtFieldName(s string, fixUpper bool) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if fixUpper && uppercaseFixups[strings.ToLower(last)] {
			parts[len(parts)-1] = strings.ToUpper(last)
		}
	}
	assembled := strings.Join(parts, "")
	runes := []rune(assembled)
	for i, c := range runes {
		ok := unicode.IsLetter(c) || unicode.IsDigit(c)
		if i == 0 {
			ok = unicode.IsLetter(c)
		}
		if !ok {
			runes[i] = '_'
		}
	}
	return string(runes)
}

// generate an appropriate struct type entry
func typeForValue(value interface{}) string {
	//Check if this is an array
	if objects, ok := value.([]interface{}); ok {
		types := make(map[reflect.Type]bool, 0)
		for _, o := range objects {
			types[reflect.TypeOf(o)] = true
		}
		if len(types) == 1 {
			return "[]" + typeForValue(objects[0])
		}
		return "[]interface{}"
	} else if object, ok := value.(map[string]interface{}); ok {
		return generateTypes(object, 0) + "}"
	} else if reflect.TypeOf(value) == nil {
		return "interface{}"
	}
	return reflect.TypeOf(value).Name()
}

// Return true if os.Stdin appears to be interactive
func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fileInfo.Mode()&(os.ModeCharDevice|os.ModeCharDevice) != 0
}

func main() {
	flag.Parse()

	if isInteractive() {
		flag.Usage()
		fmt.Fprintln(os.Stderr, "Expects input on stdin")
		os.Exit(1)
	}

	jsonStyle = Style(*jsonStyleString)
	bsonStyle = Style(*bsonStyleString)

	if !IsValidStyle(jsonStyle) {
		fmt.Fprintln(os.Stderr, `Invalid json style: "%s"`, jsonStyle)
	}

	if !IsValidStyle(bsonStyle) {
		fmt.Fprintln(os.Stderr, `Invalid bson style: "%s"`, bsonStyle)
	}

	if output, err := generate(os.Stdin, *name, *pkg); err != nil {
		fmt.Fprintln(os.Stderr, "error parsing", err)
		os.Exit(1)
	} else {
		fmt.Print(string(output))
	}
}
