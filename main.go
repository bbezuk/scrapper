// scrapper project main.go
// Idea is to load product page from url, or cached file on disk, and parse required data about produt, and then to output it to json format
package main

import (
	"bytes"
	"code.google.com/p/go-html-transform/h5"             //main library that enables parsing of html, a bit crude, biggest problem is missing support for nested selectors, jquery style, easy to implement
	"code.google.com/p/go-html-transform/html/transform" // usable only for selctors
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//Selectable is structure for parts of product that are selectable
type Selectable struct {
	Name    string
	Options []string
}

//Help structure to make json encoding easy
type Result struct {
	Name         string
	Category     string
	Features     map[string]string
	Images       []string
	Descriptions []string
	Extras       []Selectable
	Details      map[string]string
	Geometry     string
}

//flags to enable flexible work with script, you can change url, output, enable cached or non cached parsing, parsing multiple product with starting end ending id, or single with specified id
var single = flag.Uint64("single", 0, "Ako je potrebno izvršiti program za samo jedan proizvod, npr 1200, moze se dodati ovaj flag")
var min = flag.Uint64("min", 1, "Pocetni proizvod od kojeg krece scrapanje")
var max = flag.Uint64("max", 2100, "Zadnji proizvod, atm je to 2100")
var cached = flag.Bool("cached", false, "Ako je --cached postavljeno program ce iz cachea na disku probati parsati podatke, inace ponovo skida")
var url = flag.String("url", "http://example.com/product.php?id_product=", "Url zastavica -url postavlja url format za program, trebao bi biti kompletan url osim ID dijela,\n Default je: 'http://keindl-sport.hr/product.php?id_product='\n")
var output = flag.String("out", "Diff.json", "Ime file-a u koji se outputa rezultat")

func init() {
	flag.Parse()
}

//Loads html file from url, with core http library, returns it in h5.node format, very useful for latter parsing
//In this case whole html file is node, but any later selections will also be nodes.
func GetHtmlNodeFromUrl(url string, filename string) (node *h5.Node) {
	fmt.Printf("Getting data from url\n")
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error getting valid response from url: %s\n", url)
	}
	defer res.Body.Close()

	buffer, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Could not read from reader\n")
		return
	}

	p := h5.NewParserFromString(string(buffer))

	err = p.Parse()
	if err != nil {
		log.Fatalf("Error parsing body as html: %s", err)
	}

	node = p.Tree()

	SaveHtmlNodeToFile(buffer, filename)

	return
}

//This function gets html from file on disk
func GetHtmlNodeFromFile(filename string) (node *h5.Node) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}

	p := h5.NewParserFromString(string(content))

	err = p.Parse()
	if err != nil {
		log.Printf("Error: %s\n", err)
		return nil
	}

	node = p.Tree()

	return

}

func CheckOnDisc(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}
	return false
}

func SaveHtmlNodeToFile(content []byte, filename string) {
	fmt.Printf("Filename: %s\n", filename)

	err := ioutil.WriteFile(filename, content, 0777)
	if err != nil {
		log.Fatalf("Error writing file: %s", err)
	}
}

//Main parsing and encoding happens here, each part of html that requires parsing has its own function
//Downside is that if anything changes online in site display, function has to be changed also
func GetData(node *h5.Node) ([]byte, error) {

	var r Result
	var buffer bytes.Buffer

	r.Name = ParseTitle(node)

	if r.Name == "No match" {
		return []byte{}, errors.New("Not a valid product")
	}

	r.Category = ParseCategory(node)

	r.Features = ParseFeatures(node)

	r.Images = ParseImages(node)

	r.Descriptions = ParseDescriptions(node)

	r.Extras = ParseExtras(node)

	r.Details = ParseDetails(node)

	r.Geometry = ParseGeometry(node)

	j := json.NewEncoder(&buffer)

	err := j.Encode(r)

	if err != nil {
		log.Printf("Error encoding to json : %s\n", err)

		return []byte{}, err
	}

	return buffer.Bytes(), nil
}

//Main function that gets everything when you specify an id
func GetProduct(productId uint64) ([]byte, error) {
	var node *h5.Node

	completeUrl := fmt.Sprintf("%s%d", *url, productId)
	filename := fmt.Sprintf("data/product_%d.txt", productId)

	if *cached {
		if CheckOnDisc(filename) {
			node = GetHtmlNodeFromFile(filename)
		}

		if node == nil {
			node = GetHtmlNodeFromUrl(completeUrl, filename)
			time.Sleep(1 * time.Second)
		}
	} else {
		node = GetHtmlNodeFromUrl(completeUrl, filename)
		time.Sleep(1 * time.Second)
	}

	if node == nil {
		return []byte{}, errors.New(fmt.Sprintf("Could not parse node from product id: %d", productId))
	}

	data, err := GetData(node)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

//Function to parse title, all other parsing functions are similar and won't be explained, 
//Important thing to note is unescaping of string at the end, since it is html we are parsing after all
func ParseTitle(node *h5.Node) string {
	selector := []string{"#primary_block", "h2"}
	tNode := node
	var result []*h5.Node

	for _, sel := range selector {
		query := transform.NewSelectorQuery(sel)
		result = query.Apply(tNode)
		if len(result) < 1 {
			return "No match"
		}
		tNode = result[0]

	}

	return html.UnescapeString(result[0].Children[0].Data())

}

func CheckForEditable(node []*h5.Node) ([]*h5.Node, bool) {
	query := transform.NewSelectorQuery(".editable")

	result := query.Apply(node[0])

	if len(result) == 1 {
		return result, true
	}

	return node, false
}

func CheckForAvailable(node *h5.Node) (bool, bool) {
	query := transform.NewSelectorQuery(".not_available")

	result := query.Apply(node)

	if len(result) == 1 {
		return true, false
	}

	query = transform.NewSelectorQuery(".available")

	result = query.Apply(node)

	if len(result) == 1 {
		return true, true
	}

	return false, false

}

func ExtractFeature(node *h5.Node) (key, val string) {
	query := transform.NewSelectorQuery(".feature_name")

	result := query.Apply(node)

	key = html.UnescapeString(result[0].Children[0].Data())

	query = transform.NewSelectorQuery(".feature_value")

	result = query.Apply(node)

	result, editable := CheckForEditable(result)

	if editable {
		val = result[0].Children[0].Data()

		return
	}

	available, status := CheckForAvailable(result[0])

	if available {
		if status {
			val = "Dostupno"
		} else {
			val = "Nedostupno"
		}

		return
	}

	val = html.UnescapeString(result[0].Children[0].Data())
	val = strings.Trim(val, " \t\n\r")

	return
}

func ParseFeatures(node *h5.Node) (list map[string]string) {
	selector := []string{".product_short_features_list nuc_short_festures_list", "table", "tbody", "tr"}
	tNode := node
	var result []*h5.Node
	list = make(map[string]string)

	for _, sel := range selector {
		query := transform.NewSelectorQuery(sel)

		result = query.Apply(tNode)

		if len(result) < 1 {
			return nil
		}

		tNode = result[0]
	}

	for _, res := range result {
		key, val := ExtractFeature(res)
		list[key] = val
	}

	return
}

func ParseThumbs(node *h5.Node, url *string) (list []string) {
	query := transform.NewSelectorQuery("img")
	result := query.Apply(node)

	var baseSrc string

	for _, res := range result {
		for _, attr := range res.Attr {
			if attr.Name == "src" {
				baseSrc = attr.Value
			}
		}

		list = append(list, fmt.Sprintf("%s%s", *url, baseSrc))
		largeSrc := strings.Replace(baseSrc, "medium", "thickbox", 1)
		list = append(list, fmt.Sprintf("%s%s", *url, largeSrc))
	}

	return
}

func ParseImages(node *h5.Node) (list []string) {
	var coreUrl string = "http://keindl-sport.hr"

	var baseSrc string
	query := transform.NewSelectorQuery("#bigpic")
	result := query.Apply(node)

	if len(result) > 0 {
		for _, attr := range result[0].Attr {
			if attr.Name == "src" {
				baseSrc = attr.Value
			}
		}

		list = append(list, fmt.Sprintf("%s%s", coreUrl, baseSrc))
		largeSrc := strings.Replace(baseSrc, "large", "thickbox", 1)
		list = append(list, fmt.Sprintf("%s%s", coreUrl, largeSrc))
	}

	query = transform.NewSelectorQuery("#thumbs_list_frame")
	result = query.Apply(node)

	if len(result) > 0 {
		list = append(list, ParseThumbs(result[0], &coreUrl)...)
	}

	return
}

func ParseCategory(node *h5.Node) string {
	query := transform.NewSelectorQuery(".navigation_end")
	result := query.Apply(node)

	if len(result) < 1 {
		return ""
	}

	return html.UnescapeString(result[0].Children[0].Children[0].Data())
}

func ExtractSpecialDescription(sets []*h5.Node) string {
	var parts []string

	for _, set := range sets {
		if set.Type == 0 {
			parts = append(parts, set.Data())
		}
		if set.Type == 1 {
			if len(set.Children) > 0 && set.Children[0].Type == 0 {
				parts = append(parts, set.Children[0].Data())
			}
		}
	}

	return html.UnescapeString(strings.Join(parts, ""))
}

func ParseDescriptions(node *h5.Node) (list []string) {
	query := transform.NewSelectorQuery("#idTab1")
	result := query.Apply(node)

	if len(result) < 1 {
		return
	}

	for _, chapter := range result[0].Children {
		var data string
		query = transform.NewSelectorQuery("span")
		med := query.Apply(chapter)

		if len(med) < 1 {
			continue
		}

		if len(med[0].Children) > 1 {
			data = ExtractSpecialDescription(med[0].Children)
		} else {
			data = med[0].Children[0].Data()
		}

		list = append(list, data)

	}

	return
}

func ExtractExtraName(node *h5.Node) string {
	if node.Children[1].Children[0].Type == 0 {
		return strings.Trim(html.UnescapeString(node.Children[1].Children[0].Data()), ": ")
	}

	return ""
}
func ExtractExtraVals(node *h5.Node) (list []string) {
	//fmt.Printf("Test: %v\n",node.Children[3].Children)

	for _, opt := range node.Children[3].Children {
		if len(opt.Children) == 0 {
			continue
		}

		list = append(list, html.UnescapeString(opt.Children[0].Data()))

	}

	return
}

func ParseExtras(node *h5.Node) (list []Selectable) {
	query := transform.NewSelectorQuery("#attributes")
	result := query.Apply(node)

	if len(result) != 1 {
		return
	}

	for _, set := range result[0].Children {

		if len(set.Children) == 0 {
			continue
		}

		name := ExtractExtraName(set)
		opts := ExtractExtraVals(set)

		sel := Selectable{Name: name, Options: opts}

		list = append(list, sel)

	}

	return
}

func ExtractDetail(node *h5.Node) (key, val string) {
	query := transform.NewSelectorQuery(".product_feature_name")

	result := query.Apply(node)

	key = strings.Trim(html.UnescapeString(result[0].Children[0].Data()), " :\t\n\r")

	query = transform.NewSelectorQuery(".product_feature_value")

	result = query.Apply(node)

	val = html.UnescapeString(result[0].Children[0].Data())
	val = strings.Trim(val, " :\t\n\r")

	return
}

func ParseDetails(node *h5.Node) (list map[string]string) {
	list = make(map[string]string)

	query := transform.NewSelectorQuery("#idTab2")
	result := query.Apply(node)

	if len(result) > 0 {
		query = transform.NewSelectorQuery("tr")
		row := query.Apply(result[0])

		for _, res := range row {
			key, val := ExtractDetail(res)
			list[key] = val
		}
	}

	return
}

func ParseGeometry(node *h5.Node) string {
	query := transform.NewSelectorQuery("#geometry_image")
	result := query.Apply(node)
	var baseSrc string

	if len(result) > 0 {
		for _, attr := range result[0].Children[0].Attr {
			if attr.Name == "src" {
				baseSrc = attr.Value
			}
		}

		return html.UnescapeString(baseSrc)
	}

	return ""
}

//function that writes output to file
func WriteJsonOutput(data []byte) {
	err := ioutil.WriteFile(fmt.Sprintf("out/%s", *output), data, 0777)
	if err != nil {
		log.Fatalf("Error writing file: %s", err)
	}
}

//main
func main() {
	if *cached {
		fmt.Printf("Cached je postavljen. \n")
	} else {
		fmt.Printf("Cached nije postavljen \n")
	}

	var chunks bytes.Buffer
	chunks.Write([]byte("{\"Products\":["))

	if *single > 0 {
		log.Printf("Doing single product with id: %d\n", *single)

		data, err := GetProduct(*single)

		if err != nil {
			return
		}

		chunks.Write(data)

		chunks.Write([]byte("]}"))

		WriteJsonOutput(chunks.Bytes())
		return
	}

	for i := *min; i <= *max; i++ {
		log.Printf("Getting product with id: %d", i)
		data, err := GetProduct(i)

		if err != nil {
			continue
		}

		chunks.Write(data)

		if i < *max {
			chunks.Write([]byte(","))
		}

	}

	chunks.Write([]byte("]}"))

	WriteJsonOutput(chunks.Bytes())
}
