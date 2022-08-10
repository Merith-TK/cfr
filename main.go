package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// Jar Flag is the path to the jar file
	Jar = flag.String("jar", "", "Path to the jar file to pull classes from")
	Out = flag.String("out", "./src", "Path to the output directory")

	// java is the path to the java executable
	java string

	tmpDir string

	class = flag.Arg(0)
)

func main() {
	// check if cfr.jar is in the current directory
	if _, err := os.Stat("cfr.jar"); os.IsNotExist(err) {
		fmt.Println("cfr.jar not found in current directory")
		return
	}
	var err error
	// check if java is installed in path
	java, err = exec.LookPath("java")
	if err != nil {
		log.Fatalln("ERROR: NO JAVA INSTALLED, please install java")
		return
	}
	err = nil

	fmt.Println("java found in path")
	fmt.Println("java:", java)

	flag.Parse()
	if *Jar == "" {
		log.Fatalln("ERROR: NO JAR FILE PROVIDED")
		return
	} else {
		// verify jar file exists
		if _, err := os.Stat(*Jar); os.IsNotExist(err) {
			log.Fatalln("ERROR: JAR FILE DOES NOT EXIST")
			return
		}
	}

	// create temporary directory to extract jar file to
	tmpDir, err = ioutil.TempDir("", "cfr")
	if err != nil {
		log.Fatalln("ERROR: UNABLE TO CREATE TEMPORARY DIRECTORY")
		return
	}
	Unzip(*Jar, tmpDir)
	fmt.Println("unzipped jar file to:", tmpDir)

	// create output directory if it doesn't exist
	if _, err := os.Stat(*Out); os.IsNotExist(err) {
		os.Mkdir(*Out, 0755)
	}
	fmt.Println("output directory:", *Out)
	fmt.Println("")
	fmt.Println("extracting classes...")

	// get all files in temporary directory and subdirectories
	traverse(tmpDir)
}

func traverse(temppath string) {
	// get all files in temporary directory and subdirectories
	files, err := ioutil.ReadDir(temppath)
	if err != nil {
		log.Fatalln("ERROR: UNABLE TO READ DIRECTORY")
		return
	}
	for _, f := range files {
		// get full path to file
		fullPath := filepath.Join(temppath, f.Name())
		// if file is a directory, traverse into it
		if f.IsDir() {
			traverse(fullPath)
		} else {
			// if file is a .class file, extract it
			//if strings.HasSuffix(fullPath, ".class") {
			if !strings.Contains(fullPath, "$") {
				extract(fullPath, class, *Out, tmpDir)
			}
			// } else {
			// 	fmt.Println("skipping:", fullPath)
			// }
		}
	}
}

func extract(path string, class string, out string, tempDir string) {
	// replace \ with /
	path = strings.Replace(path, "\\", "/", -1)
	tempDir = strings.Replace(tempDir, "\\", "/", -1)

	out = strings.Replace(path, tempDir, out, 1)

	// remove last part of path (.class)
	outDir := strings.Replace(out, filepath.Base(path), "", 1)
	// create output directory if it doesn't exist
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		fmt.Println("creating output directory:", outDir)
		err := os.MkdirAll(outDir, 0755)
		if err != nil {
			log.Fatalln("ERROR: UNABLE TO CREATE OUTPUT DIRECTORY", err)
			return
		}
	}

	out = strings.Replace(out, ".class", ".java", 1)
	// check if file already exists
	if _, err := os.Stat(out); os.IsNotExist(err) {

		if strings.HasSuffix(out, ".java") {
			// run java -jar cfr.jar <path> <class>  and write output to file
			cmd := exec.Command(java, "-jar", "cfr.jar", path, class)
			outPut, err := cmd.Output()
			if err != nil {
				log.Fatalln("ERROR: UNABLE TO EXTRACT CLASS")
				return
			}

			// write output to file
			err = ioutil.WriteFile(out, outPut, 0644)
			if err != nil {
				log.Fatalln("ERROR: UNABLE TO WRITE OUTPUT TO FILE", err)
				return
			}
		} else {
			// copy file to output directory
			_, err := copyFile(path, out)
			if err != nil {
				log.Fatalln("ERROR: UNABLE TO COPY FILE", err)
				return
			}
		}
	}

}

func copyFile(in, out string) (int64, error) {
	i, e := os.Open(in)
	if e != nil {
		return 0, e
	}
	defer i.Close()
	o, e := os.Create(out)
	if e != nil {
		return 0, e
	}
	defer o.Close()
	return o.ReadFrom(i)
}

// https://stackoverflow.com/questions/20357223/easy-way-to-unzip-file-with-golang#24792688
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
