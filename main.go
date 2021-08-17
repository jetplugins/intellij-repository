package main

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

func main() {
	var port int
	var workdir, defaultFile, domain string
	flag.IntVar(&port, "p", 80, "intellij-repository -p 80")
	flag.StringVar(&workdir, "d", "./", "intellij-repository -d ./")
	flag.StringVar(&defaultFile, "df", "", "intellij-repository -df plugins.xml")
	flag.StringVar(&domain, "domain", "", "intellij-repository -domain http://your.com")
	flag.Parse()
	if defaultFile == "" {
		initPluginsXml(workdir)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Path[1:]
		if r.URL.Path == "/" {
			file = defaultFile
			if defaultFile == "" {
				writePluginXml(w, r, domain)
				return
			}
		}
		http.ServeFile(w, r, path.Join(workdir, file))
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Printf("[ERROR] Start error: %s\n", err)
		return
	}
	log.Println("[INFO] Stopped.")
}

//-------------------------------------------------------------------------------
// auto generate plugins.xml
//-------------------------------------------------------------------------------

var xmlTpl string     // plugins.xml数据模板
var xmlCache sync.Map // plugins.xml的内容
const RepositoryUrlFlag = "{INTELLIJ_REPOSITORY_URL}"

// Write plugins xml data to response.
func writePluginXml(w http.ResponseWriter, r *http.Request, domain string) {
	if domain == "" {
		domain = "http://" + r.Host + r.URL.Path
		if domain[len(domain)-1:] == "/" {
			domain = domain[:len(domain)-1]
		}
	}
	content := getPluginsXml(domain)
	w.Header().Set("Content-TYPE", "application/xml;utf-8")
	_, err := w.Write(content)
	if err != nil {
		log.Printf("[ERROR] Write content error: %s", err)
	}
}

// Generate plugins xml content from plugin file.
func initPluginsXml(dir string) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	// find plugins
	files := findPluginFile(dir, 3, 0)
	plugins := make([]*RepoPlugin, 0)
	for _, file := range files {
		log.Printf("[INFO] Scan plugin: %s\n", file)
		plugin, err := resolvePluginFile(file)
		if err != nil {
			log.Printf("[ERROR] Parse plugin error: file=%s, error=%s", file, err)
			continue
		}
		if plugin != nil {
			plugin.Url = RepositoryUrlFlag + file[len(dir):]
			plugins = append(plugins, plugin)
		}
	}
	// generate xml
	repoPlugins := &RepoPlugins{Plugins: plugins}
	repoPlugins.Comment = "Generated At: " + time.Now().Format("2006-01-02 15:04")
	xmlBytes, err := xml.MarshalIndent(repoPlugins, "", "    ")
	if err != nil {
		panic(err)
	}
	xmlTpl = string(xmlBytes)
}

func findPluginFile(dir string, maxIdx int, idx int) []string {
	files := make([]string, 0, 10)
	if idx > maxIdx {
		return files
	}
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Printf("[ERROR] Scan plugins dir: dir=%s, error=%s\n", dir, err)
	}
	for _, fileInfo := range fileInfos {
		if strings.Index(fileInfo.Name(), ".") == 0 {
			continue
		}
		filePath := path.Join(dir, fileInfo.Name())
		if fileInfo.IsDir() {
			subFiles := findPluginFile(filePath, maxIdx, idx+1)
			files = append(files, subFiles...)
			continue
		}
		ext := path.Ext(filePath)
		if ext == ".jar" || ext == ".zip" {
			files = append(files, filePath)
		}
	}
	return files
}

// 获取当前请求的plugins.xml
func getPluginsXml(url string) []byte {
	value, ok := xmlCache.Load(url)
	if ok {
		return value.([]byte)
	}
	content := []byte(strings.ReplaceAll(xmlTpl, RepositoryUrlFlag, url))
	xmlCache.Store(url, content)
	return content
}

// 从插件包解析信息
func resolvePluginFile(file string) (*RepoPlugin, error) {
	ext := path.Ext(file)
	if ext == ".jar" {
		return resolvePluginFileJar(file)
	} else {
		return resolvePluginFileZip(file)
	}
}

// 解析ZIP格式插件包
func resolvePluginFileZip(file string) (*RepoPlugin, error) {
	reader, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// 查找zip中的插件jar路径
	pluginJarRegexp := regexp.MustCompile("^(?P<name1>.+)/lib/(?P<name2>.+)\\.jar$")
	var pluginJar *zip.File
	for _, f := range reader.File {
		matches := pluginJarRegexp.FindStringSubmatch(f.Name)
		if len(matches) >= 3 && strings.Index(matches[2], matches[1]) == 0 {
			pluginJar = f
			break
		}
	}
	if pluginJar == nil {
		return nil, errors.New("not found plugin jar in archive file")
	}
	// 解压插件到指定位置
	dstFilePath := path.Join("/tmp/intellij-repository", pluginJar.Name)
	if err := os.MkdirAll(filepath.Dir(dstFilePath), os.ModePerm); err != nil {
		return nil, err
	}
	f, err := pluginJar.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dstFile, err := os.OpenFile(dstFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, pluginJar.Mode())
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(dstFile, f); err != nil {
		return nil, err
	}

	return resolvePluginFileJar(dstFilePath)
}

// 从插件包Jar格式插件包
func resolvePluginFileJar(file string) (*RepoPlugin, error) {
	reader, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var theFile *zip.File
	for _, f := range reader.File {
		if f.Name == "META-INF/plugin.xml" {
			theFile = f
			break
		}
	}
	if theFile == nil {
		return nil, nil
	}

	f, err := theFile.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	xmlBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	packPlugin := new(PackPlugin)
	err = xml.Unmarshal(xmlBytes, packPlugin)
	if err != nil {
		return nil, err
	}
	repoPlugin := &RepoPlugin{
		Id:          packPlugin.Id,
		Version:     packPlugin.Version,
		Url:         "",
		Name:        packPlugin.Name,
		ChangeNotes: "<![CDATA[" + packPlugin.ChangeNotes + "]]>",
	}
	if repoPlugin.Id == "" {
		repoPlugin.Id = repoPlugin.Name
	}
	return repoPlugin, nil
}

type RepoPlugins struct {
	XMLName xml.Name      `xml:"plugins"`
	Comment string        `xml:",comment"`
	Plugins []*RepoPlugin `xml:"plugin"`
}

type RepoPlugin struct {
	Id          string `xml:"id,attr"`
	Version     string `xml:"version,attr"`
	Url         string `xml:"url,attr"`
	Name        string `xml:"name"`
	ChangeNotes string `xml:"change-notes"`
}

type PackPlugin struct {
	Id          string `xml:"id"`
	Name        string `xml:"name"`
	Version     string `xml:"version"`
	ChangeNotes string `xml:"change-notes"`
}
