package main

import (
	"bufio"
	"fmt"
	"golang.org/x/term"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const ServersConfig = "./servers.csv"
const CommandsConfig = "./commands.csv"

var username, sshKey, port string = "", "", ""
var passphrase []byte

type serverConfig struct {
	IP       string
	Port     string
	Username string
	KeyPath  string
}

var subFolderPath string
var wg sync.WaitGroup

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("\r\nFatal: %s \n\n", err)
			os.Exit(1)
		}
	}()

	// Получение парольной фразы
	fmt.Printf("Enter passphrase for all ssh keys: ")
	passphr, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Не удалось считать парольную фразу: ")
		return
	}
	passphrase = passphr
	fmt.Println("\n")

	err = os.Mkdir("logs", 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	currentTime := time.Now()
	subfolderName := currentTime.Format("2006-01-02_15-04-05")
	subFolderPath = filepath.Join("logs", subfolderName)
	err = os.Mkdir(subFolderPath, 0755)
	if err != nil {
		log.Fatal(err)
	}

	filePath := filepath.Join(subFolderPath, "logfile.log")
	logFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	errfilePath := filepath.Join(subFolderPath, "errorLogfile.log")
	errLogFile, err := os.OpenFile(errfilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer errLogFile.Close()

	var commands []string
	var config []serverConfig

	file, err := os.Open(ServersConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		server, err := parseServerConfigLine(line)
		if err != nil && err.Error() == "Ignored comment line" {
			continue
		}
		if err != nil {
			fmt.Println("Error occurred while parsing server configuration:", err)
			continue
		}
		config = append(config, server)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	commands, _ = getCommandsFromConfig(CommandsConfig)

	outchan := make(chan string, len(config))
	errchan := make(chan string, len(config))

	for _, host := range config {
		wg.Add(1)
		go connectingRemoteServers(host, commands, outchan, errchan)
	}

	go func() {
		wg.Wait()
		close(outchan)
		close(errchan)
	}()

	multi := io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(multi)

	for out := range outchan {
		log.Println(out)
	}

	for errout := range errchan {
		// Пишем вывод напрямую в файл ошибок
		fmt.Fprintf(errLogFile, errout)
	}

	log.Println("\n")
}
