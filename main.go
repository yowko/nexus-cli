package main

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/urfave/cli"
	"github.com/yowko/nexus-cli/registry"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	CREDENTIALS_TEMPLATES = `# Nexus Credentials
nexus_host = "{{ .Host }}"
nexus_username = "{{ .Username }}"
nexus_password = "{{ .Password }}"
nexus_repository = "{{ .Repository }}"`
)

func main() {
	app := cli.NewApp()
	app.Name = "Nexus CLI"
	app.Usage = "Manage Docker Private Registry on Nexus"
	app.Version = "1.1.0"
	app.Authors = []cli.Author{
		{
			Name:  "Yowko",
			Email: "yowko@yowko.com",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "configure",
			Usage: "Configure Nexus Credentials",
			Action: func(c *cli.Context) error {
				return setNexusCredentials(c)
			},
		},
		{
			Name:  "image",
			Usage: "Manage Docker Images",
			Subcommands: []cli.Command{
				{
					Name:  "ls",
					Usage: "List all images in repository",
					Action: func(c *cli.Context) error {
						return listImages(c)
					},
				},
				{
					Name:  "tags",
					Usage: "Display all image tags",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name, n",
							Usage: "List tags by image name",
						},
						cli.StringFlag{
							Name:  "sort, s",
							Usage: "Sort tags by semantic version default, assuming all tags are semver except latest.",
						},
					},
					Action: func(c *cli.Context) error {
						return listTagsByImage(c)
					},
				},
				{
					Name:  "info",
					Usage: "Show image details",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name: "name, n",
						},
						cli.StringFlag{
							Name: "tag, t",
						},
					},
					Action: func(c *cli.Context) error {
						return showImageInfo(c)
					},
				},
				{
					Name:  "delete",
					Usage: "Delete an image",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name: "name, n",
						},
						cli.StringFlag{
							Name: "tag, t",
						},
						cli.StringFlag{
							Name: "keep, k",
						},
						cli.StringFlag{
							Name: "sort, s",
						},
						cli.StringFlag{
							Name:  "exclude, e",
							Usage: "Exclude specific tags",
						},
					},
					Action: func(c *cli.Context) error {
						return deleteImage(c)
					},
				},
			},
		},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		fmt.Fprintf(c.App.Writer, "Wrong command %q !", command)
	}
	app.Run(os.Args)
}

func setNexusCredentials(c *cli.Context) error {
	var hostname, repository, username, password string
	fmt.Print("Enter Nexus Host: ")
	fmt.Scan(&hostname)
	fmt.Print("Enter Nexus Repository Name: ")
	fmt.Scan(&repository)
	fmt.Print("Enter Nexus Username: ")
	fmt.Scan(&username)
	fmt.Print("Enter Nexus Password: ")
	fmt.Scan(&password)

	data := struct {
		Host       string
		Username   string
		Password   string
		Repository string
	}{
		hostname,
		username,
		password,
		repository,
	}

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	filePath, _ := filepath.Abs(exPath + "/.credentials")
	fmt.Println(filePath)

	tmpl, err := template.New(filePath).Parse(CREDENTIALS_TEMPLATES)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	err = tmpl.Execute(f, data)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	return nil
}

func listImages(c *cli.Context) error {
	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	images, err := r.ListImages()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	for _, image := range images {
		fmt.Println(image)
	}
	fmt.Printf("Total images: %d\n", len(images))
	return nil
}

func listTagsByImage(c *cli.Context) error {
	var imgName = c.String("name")
	var sort = c.String("sort")
	var excludes = strings.Split(c.String("exclude"), ",")
	var excludeShas []string
	if sort != "nosemver" {
		sort = "semver"
	}
	excludes = append(excludes, "latest")

	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if imgName == "" {
		cli.ShowSubcommandHelp(c)
	}
	tags, err := r.ListTagsByImage(imgName)

	for i, val := range excludes {
		fmt.Printf("%d\t%s\n", i, val)
		excludesha, err := r.GetImageSHA(imgName, val)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		fmt.Println(excludesha)
		excludeShas = append(excludeShas, excludesha)
	}

	compareStringNumber := getSortComparisonStrategy(imgName, sort, excludeShas)
	Compare(compareStringNumber).Sort(tags)

	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	for _, tag := range tags {
		fmt.Println(tag)
	}
	fmt.Printf("There are %d images for %s\n", len(tags), imgName)
	return nil
}

func showImageInfo(c *cli.Context) error {
	var imgName = c.String("name")
	var tag = c.String("tag")
	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if imgName == "" || tag == "" {
		cli.ShowSubcommandHelp(c)
	}
	manifest, err := r.ImageManifest(imgName, tag)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Printf("Image: %s:%s\n", imgName, tag)
	fmt.Printf("Size: %d\n", manifest.Config.Size)
	fmt.Println("Layers:")
	for _, layer := range manifest.Layers {
		fmt.Printf("\t%s\t%d\n", layer.Digest, layer.Size)
	}
	return nil
}

func contains(stringSlice []string, searchString string) bool {
	if len(stringSlice) > 0 {
		for _, value := range stringSlice {
			if value == searchString {
				return true
			}
		}
	}
	return false
}

func executeDelete(keep int, imgName string, excludeShas []string, targetTags []string) error {
	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	for i, tag := range targetTags {
		if i < len(targetTags)-keep {

			fmt.Printf("%s:%s image will be deleted ...\n", imgName, tag)
			tmpSha, err := r.GetImageSHA(imgName, tag)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			if !contains(excludeShas, tmpSha) {
				r.DeleteImageByTag(imgName, tag)
			} else {
				fmt.Printf("exclude %s when tag is %s\n", tmpSha, tag)
			}
		}
	}

	return nil
}
func deleteImage(c *cli.Context) error {
	var imgName = c.String("name")
	var tag = c.String("tag")
	var keeps = strings.Split(c.String("keep"), ",")
	var excludes = strings.Split(c.String("exclude"), ",")
	var sort = c.String("sort")
	var excludeShas []string
	if sort != "nosemver" {
		sort = "semver"
	}

	excludes = append(excludes, "latest")

	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	var keepList = make(map[string]*keepObject)
	// prod 預設全部保留
	keepList["prod"] = &keepObject{keepCount: 999}

	for _, val := range keeps {
		//fmt.Printf("%d\t%s\n", i, val)
		var tmpTag string
		tmpKeep := 999
		if strings.Contains(val, ":") {
			tmpVal := strings.Split(val, ":")
			tmpTag = tmpVal[0]
			tmpKeep, err = strconv.Atoi(tmpVal[1])
		} else {
			tmpTag = val
		}
		keepList[tmpTag] = &keepObject{keepCount: tmpKeep}
	}
	//建立未指定要 keep 的 container
	keepList["others"] = &keepObject{keepCount: 0}

	for _, val := range excludes {
		//fmt.Printf("%d\t%s\n", i, val)

		excludesha, err := r.GetImageSHA(imgName, val)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		//fmt.Println(excludesha)
		excludeShas = append(excludeShas, excludesha)
	}
	if imgName == "" {
		fmt.Fprintf(c.App.Writer, "You should specify the image name\n")
		cli.ShowSubcommandHelp(c)
	} else {
		if tag == "" {

			tags, err := r.ListTagsByImage(imgName)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			for _, tag := range tags {
				//fmt.Printf("%d\t%s\n", i, tag)
				//符合要 exclude 的就不處理
				if !contains(excludes, tag) {

					var tmpPrefix string
					if strings.Contains(tag, "-") {
						tmpVal := strings.Split(tag, "-")
						tmpPrefix = tmpVal[0]
					} else {
						tmpPrefix = tag

					}
					if val, ok := keepList[tmpPrefix]; ok {
						val.tags = append(val.tags, tag)
					} else {
						keepList["others"].tags = append(keepList["others"].tags, tag)
					}

				}
			}
			for _, v := range keepList {
				excludeShas, err = deleteTargetTags(imgName, v.tags, v.keepCount, sort, excludeShas)
			}

		} else {
			err = r.DeleteImageByTag(imgName, tag)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
		}
	}
	return nil
}

type keepObject struct {
	keepCount int
	tags      []string
}

func deleteTargetTags(imgName string, targetTags []string, keepCount int, sort string, excludeShas []string) ([]string, error) {

	if len(targetTags) > 0 && keepCount < 999 {
		if len(targetTags) < keepCount {
			keepCount = len(targetTags)
		}
		r, err := registry.NewRegistry()
		if err != nil {
			return excludeShas, cli.NewExitError(err.Error(), 1)
		}
		if keepCount >= 0 {

			compareStringNumber := getSortComparisonStrategy(imgName, sort, excludeShas) //excludes)

			if compareStringNumber != nil {
				Compare(compareStringNumber).Sort(targetTags)
			}
		}
		startIndex := len(targetTags) - keepCount + 1
		if startIndex < 0 || startIndex > len(targetTags) {
			startIndex = 0
		}
		//fmt.Println(startIndex)
		//fmt.Println(len(targetTags))
		if keepCount > 0 {
			for _, tmpTag := range targetTags[startIndex:] {
				tmpsha, err := r.GetImageSHA(imgName, tmpTag)
				if err != nil {
					return excludeShas, cli.NewExitError(err.Error(), 1)
				}
				//fmt.Printf("index:%d, imageName:%s,tag:%s \n", i, imgName, tmpTag)
				excludeShas = append(excludeShas, tmpsha)
			}
		}

		err = executeDelete(keepCount, imgName, excludeShas, targetTags)
		if err != nil {
			return excludeShas, cli.NewExitError(err.Error(), 1)
		}
	}
	return excludeShas, nil
}

func getSortComparisonStrategy(imgName string, sort string, excludeShas []string) func(str1, str2 string) bool {
	var compareStringNumber func(str1, str2 string) bool

	if sort == "nosemver" {
		compareStringNumber = func(str1, str2 string) bool {
			return extractNumberFromString(str1) < extractNumberFromString(str2)
		}
	}

	if sort == "semver" {
		compareStringNumber = func(str1, str2 string) bool {
			//fmt.Printf("str1: %q\n;str2: %q\n;!contains(excludes,str1): %q\n;contains(excludes,str2): %q\n", str1,str2,!contains(excludes,str1),contains(excludes,str2))

			r, err := registry.NewRegistry()
			if err != nil {
				fmt.Printf("Error init NewRegistry: %q\n", err)
			}

			shastr1, err := r.GetImageSHA(imgName, str1)
			if err != nil {
				fmt.Printf("Error get %s:%s sha: %q\n", imgName, str1, err)
			}

			if contains(excludeShas, shastr1) {
				return false
			}

			shastr2, err := r.GetImageSHA(imgName, str2)
			if err != nil {
				fmt.Printf("Error get %s:%s sha: %q\n", imgName, str2, err)
			}

			if contains(excludeShas, shastr2) {
				return true
			}

			var semverStr1 string
			if strings.ContainsAny(str1, "-") {
				tmpSemverStr := strings.Split(str1, "-")
				semverStr1 = tmpSemverStr[1]
			} else {
				semverStr1 = str1
			}
			//fmt.Printf("get semver from string with prefix %s:%s ; \n",str1,semverStr1)

			version1, err1 := semver.Make(semverStr1)
			if err1 != nil {
				fmt.Printf("Error parsing version1: %s ; %q\n", semverStr1, err1)
			}

			var semverStr2 string
			if strings.ContainsAny(str2, "-") {
				tmpSemverStr := strings.Split(str2, "-")
				semverStr2 = tmpSemverStr[1]
			} else {
				semverStr2 = str2
			}
			//fmt.Printf("get semver from string with prefix %s:%s ;\n",str2,semverStr2)

			version2, err2 := semver.Make(semverStr2)
			if err2 != nil {
				fmt.Printf("Error parsing version2: %s ;  %q\n", semverStr2, err2)
			}

			//fmt.Printf("get semver from string with prefix str1:%s ; str2:%s\nÍ",semverStr1,semverStr2)
			return version1.LT(version2)
		}
	}

	return compareStringNumber
}
