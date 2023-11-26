package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bogem/id3v2"
)

var RemovalFiles []string

func RunCommand(app string, cmdLn string, stdin string) (bytes.Buffer, error) {
	cmd := exec.Command(app)
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLn}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	// fmt.Println(err, "\n", outb.String(), "\n", errb.String())
	if err != nil {
		newErr := errors.New(errb.String() + "\n" + err.Error())
		return bytes.Buffer{}, newErr
	}

	return outb, nil
}

func DownloadVideo(title, link string) error {
	fmt.Printf("Downloading \"%s\"...", title)

	cmdLn := fmt.Sprintf("/c -f \"ba/b\" -x --audio-format mp3 %s -o \"%s\"", link, title)
	_, err := RunCommand("yt-dlp", cmdLn, "")
	if err != nil {
		return err
	}
	RemovalFiles = append(RemovalFiles, title+".mp3")
	fmt.Println("Video successfully downloaded...")
	return nil
}

func sectionAudio(start, finish, trackName, filename, dir string) (string, error) {
	trackName = filepath.Join(dir, trackName+".mp3")
	cmdLn := fmt.Sprintf("/c -i \"%s.mp3\" -ss %s -to %s -c copy \"%s\"", filename, start, finish, trackName)

	_, err := RunCommand("ffmpeg", cmdLn, "")
	if err != nil {
		return "", err
	}
	return trackName, nil
}

func SplitVid(dir, title, link string) error {
	timeStamps, err := GetTimeStamps(title)
	if err != nil {
		return err
	}

	trackNames, err := GetTrackNames(title)
	if err != nil {
		return err
	}

	artistName, err := GetChannelName(title)
	if err != nil {
		return err
	}

	err = GetThumbNail(link, title)
	if err != nil {
		return err
	}
	fmt.Println("Splitting into tracks...")
	fmt.Print("\033[s")
	for i := 0; i < len(trackNames); i++ {
		fmt.Printf("\033[u\033[K%d out ouf %d...", i, len(trackNames))
		trackName, err := ValidateFileName(trackNames[i])
		if err != nil {
			return err
		}
		fileName, err := sectionAudio(timeStamps[i], timeStamps[i+1], trackName, title, dir)
		if err != nil {
			return err
		}
		err = AddTags(trackName, title, artistName, fileName, dir)
		if err != nil {
			return err
		}

	}
	fmt.Print("\033[u\033[T")
	fmt.Println("Finished splitting into seperate tracks!")
	return nil
}

func ValidateFileName(filename string) (string, error) {
	pattern := `^[\w\-. ]+$`
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Println("f")
		return "", err
	}
	if string(re.Find([]byte(filename))) == "" {
		pattern = `^[\w\-. ]+`
		re, err = regexp.Compile(pattern)
		if err != nil {
			return "", err
		}
		vaildPart := string(re.Find([]byte(filename)))
		nonValid, _ := strings.CutPrefix(filename, vaildPart)
		filename = vaildPart + nonValid[1:]
		return ValidateFileName(filename)
	}
	return filename, nil
}

func AddTags(trackName, albumTitle, artistName, fileName, dir string) error {
	tag, err := id3v2.Open(fileName, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	tag.SetArtist(artistName)
	tag.SetTitle(trackName)
	tag.SetAlbum(albumTitle)

	artworkPath := albumTitle + ".jpg"

	artwork, err := os.ReadFile(artworkPath)
	if err != nil {
		newErr := errors.New("Trouble opening thumbnail file:" + err.Error())
		return newErr
	}

	pic := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/jpg",
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     artwork,
	}
	tag.AddAttachedPicture(pic)

	if err = tag.Save(); err != nil {
		return err
	}

	return nil
}

func GetChannelName(title string) (string, error) {
	fmt.Println("Getting channel name...")
	cmdLn := "/c --raw-output \".channel\""
	dump, err := os.ReadFile(title + ".json")
	if err != nil {
		return "", err
	}

	buffer, err := RunCommand("jq", cmdLn, string(dump))
	if err != nil {
		return "", err
	}

	output := strings.TrimRight(buffer.String(), "\r\n")
	fmt.Println("Got channel name!")
	return output, nil
}

func SaveDumpFile(title, link string) error {
	fmt.Println("Saving dump file...")
	cmdLn := fmt.Sprintf("/c --dump-json %s", link)
	dump, err := RunCommand("yt-dlp", cmdLn, "")
	if err != nil {
		return err
	}

	f, err := os.Create(title + ".json")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(dump.String())
	if err != nil {
		return err
	}

	RemovalFiles = append(RemovalFiles, title+".json")
	fmt.Println("Dump file saved!")
	return nil
}

func GetTitle(link string) (string, error) {
	fmt.Println("Getting video title...")

	cmdLn := fmt.Sprintf("/c --dump-json %s", link)
	dump, err := RunCommand("yt-dlp", cmdLn, "")
	if err != nil {
		return "", err
	}

	cmdLn = "/c --raw-output \".title\""
	buffer, err := RunCommand("jq", cmdLn, dump.String())
	if err != nil {
		return "", err
	}

	output := strings.TrimRight(buffer.String(), "\r\n")
	title, err := ValidateFileName(output)
	if err != nil {
		return "", err
	}

	fmt.Println("Got video title!")
	return title, nil
}

func GetThumbNail(link, title string) error {
	fmt.Println("Getting channel thumbnail...")
	cmdLn := fmt.Sprintf("/c  --write-thumbnail --skip-download -o \"thumbnail:%s\" %s", title, link)
	_, err := RunCommand("yt-dlp", cmdLn, "")
	if err != nil {
		return err
	}
	RemovalFiles = append(RemovalFiles, title+".webp")

	cmdLn = fmt.Sprintf("/c -i \"%s.webp\" -filter:v \"crop=ih:ih\" \"%s.jpg\"", title, title)
	_, err = RunCommand("ffmpeg", cmdLn, "")
	if err != nil {
		return err
	}
	RemovalFiles = append(RemovalFiles, title+".jpg")

	fmt.Println("Got thumbnail!")
	return nil
}

func GetDuration(title string) (string, error) {
	cmdLn := "/c --raw-output \".duration\""

	dump, err := os.ReadFile(title + ".json")
	if err != nil {
		return "", err
	}

	buffer, err := RunCommand("jq", cmdLn, string(dump))
	if err != nil {
		return "", err
	}
	output := strings.TrimRight(buffer.String(), "\r\n")

	duration := strings.TrimSuffix(output, ".0")
	durationConverted, err := strconv.Atoi(duration)
	if err != nil {
		return "", err
	}
	durationConverted -= 1

	output = strconv.Itoa(durationConverted)

	output, err = ConvertTimeStamps(output)
	if err != nil {
		return "", err
	}
	return output, nil
}

func GetTimeStamps(title string) ([]string, error) {
	fmt.Println("Getting timestamps...")

	cmdLn := "/c --raw-output \".chapters[].start_time\""
	dump, err := os.ReadFile(title + ".json")
	if err != nil {
		return []string{}, err
	}
	stamps, err := RunCommand("jq", cmdLn, string(dump))
	if err != nil {
		return []string{}, err
	}

	output := []string{}

	scanner := bufio.NewScanner(&stamps)
	for scanner.Scan() {
		stamp, err := ConvertTimeStamps(scanner.Text())
		if err != nil {
			return []string{}, err
		}
		output = append(output, stamp)
	}

	duration, err := GetDuration(title)
	if err != nil {
		return []string{}, err
	}

	output = append(output, duration)
	fmt.Println("Successfully downloaded timestamps!")
	return output, nil
}

func GetTrackNames(title string) ([]string, error) {
	fmt.Println("Getting track names...")
	cmdLn := "/c --raw-output \".chapters[].title\""

	dump, err := os.ReadFile(title + ".json")
	if err != nil {
		return []string{}, err
	}

	outb, err := RunCommand("jq", cmdLn, string(dump))
	if err != nil {
		return []string{}, err
	}

	output := []string{}
	scanner := bufio.NewScanner(&outb)
	for scanner.Scan() {
		text := strings.TrimRight(scanner.Text(), "\r\n")
		output = append(output, text)
	}
	fmt.Println("Succsefully got track names!")
	return output, nil
}

func ConvertTimeStamps(timeStamp string) (string, error) {
	timeStamp = strings.TrimSuffix(timeStamp, ".0")

	d, err := time.ParseDuration(timeStamp + "s")
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s), nil
}

func RemoveFile() {
	for _, file := range RemovalFiles {
		err := os.Remove(file)
		if err != nil {
			panic(err)
		}
	}
}

func Bub(keepTmpFiles bool, link, defaultFolder string) error {
	title, err := GetTitle(link)
	if err != nil {
		return err
	}
	err = SaveDumpFile(title, link)
	if err != nil {
		return err
	}

	dir := filepath.Join(defaultFolder, title)

	err = os.Mkdir(dir, os.ModePerm)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	if os.IsExist(err) {
		log.Println("Folder exists, skipping...")
		return nil
	} else {
		if !keepTmpFiles {
			defer RemoveFile()
		}

		err := DownloadVideo(title, link)
		if err != nil {
			log.Fatal(err)
			return err
		}

		err = SplitVid(dir, title, link)
		if err != nil {
			log.Fatal(err)
			return err
		}

	}
	return nil
}

func main() {
	link := flag.String("l", "", "link to youtube video")
	playlistFilePath := flag.String("p", "", "-path to a playlist file")
	keepTmpFiles := flag.Bool("tmp", false, "keep downloaded files")
	flag.Parse()

	if *playlistFilePath == "" && *link == "" {
		log.Fatal("no link specified")
		return
	}

	switch {
	case *playlistFilePath != "":
		file, err := os.Open(*playlistFilePath)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer file.Close()

		defaultFolder := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		err = os.Mkdir(defaultFolder, os.ModePerm)
		if err != nil {
			if !os.IsExist(err) {
				log.Fatal(err)
				return
			}
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			err := Bub(*keepTmpFiles, scanner.Text(), defaultFolder)
			if err != nil {
				log.Fatal(err)
				return
			}
		}
	default:
		defaultFolder := "download"

		err := os.Mkdir(defaultFolder, os.ModePerm)
		if err != nil {
			if !os.IsExist(err) {
				log.Fatal(err)
				return
			}
		}

		err = Bub(*keepTmpFiles, *link, defaultFolder)
		if err != nil {
			log.Fatal(err)
			return
		}
	}

}
