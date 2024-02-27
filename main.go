package main

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const ServersConfig = "./servers.csv"
const CommandsConfig = "./commands.csv"

var username, sshKey, port string = "", "", ""

type serverConfig struct {
	IP       string
	Port     string
	Username string
	KeyPath  string
}

var subFolderPath string
var wg sync.WaitGroup

func parseServerConfigLine(line string) (serverConfig, error) {
	var config serverConfig
	if strings.HasPrefix(line, "#") || containsOnlyWhitespaceOrEmpty(line) {
		return config, fmt.Errorf("Ignored comment line")
	}

	var fields []string

	if strings.Contains(line, ",") {
		fields = strings.Split(line, ",")
	} else if strings.Contains(line, ";") || len(line) > 0 {
		fields = strings.Split(line, ";")
	} else {
		return config, fmt.Errorf("Invalid configuration format")
	}

	//if len(fields) < 3 {
	//	return config, fmt.Errorf("Invalid configuration format")
	//}
	config.IP = strings.TrimSpace(fields[0])

	if len(fields) <= 2 && username != "" && sshKey != "" && port != "" {
		config.Username = username
		config.KeyPath = sshKey
		config.Port = port
	} else if len(fields) <= 3 && sshKey != "" && port != "" {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		config.KeyPath = sshKey
		config.Port = port
	} else if len(fields) <= 4 && port != "" {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		sshKey = strings.TrimSpace(fields[2])
		config.KeyPath = strings.TrimSpace(fields[2])
		config.Port = port
	} else {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		sshKey = strings.TrimSpace(fields[2])
		config.KeyPath = strings.TrimSpace(fields[2])
		if len(fields) <= 4 || containsOnlyWhitespaceOrEmpty(fields[3]) {
			port = "22"
			config.Port = "22"
		} else {
			port = strings.TrimSpace(fields[3])
			config.Port = strings.TrimSpace(fields[3])
		}
	}

	return config, nil
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("\r\nFatal: %s \n\n", err)
			os.Exit(1)
		}
	}()

	err := os.Mkdir("logs", 0755)
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

	var commands commands
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

	commands.getCommandFromConfig(CommandsConfig)

	outchan := make(chan string, len(config))

	for _, host := range config {
		wg.Add(1)
		go runOnServer(host, commands, outchan, errLogFile)
	}

	go func() {
		wg.Wait()
		close(outchan)
	}()

	multi := io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(multi)

	for out := range outchan {
		log.Println(out)
	}

	fmt.Println("______________________________________________________________________________")
}

func runOnServer(host serverConfig, commands commands, outchan chan string, errLogFile *os.File) error {

	defer wg.Done()
	// Создание файла в подпапке
	filePath := filepath.Join(subFolderPath, string(host.IP+".log"))
	logHost, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logHost.Close()

	buffer := new(bytes.Buffer)

	fmt.Fprintf(buffer, "*****************************************************************************")
	fmt.Fprintf(buffer, "\n  ")
	fmt.Fprintf(buffer, "________________  HOST - %s  ____________________ \n\n", host.IP)

	// Загрузка закрытого ключа для аутентификации
	key, err := os.ReadFile(host.KeyPath)
	if err != nil {
		logError("Не удалось прочитать закрытый ключ: ", err, host.IP)
		fmt.Printf("Не удалось прочитать закрытый ключ: %v\n", err)
		return err
		//os.Exit(1)
	}

	// Создание Signer из закрытого ключа
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		logError("Не удалось разобрать закрытый ключ: ", err, host.IP)
		return err
	}

	// Создание структуры конфигурации SSH
	conf := &ssh.ClientConfig{
		User: host.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Не использовать в продакшене!
	}

	// Подключение к серверу
	connection, err := ssh.Dial("tcp", host.IP+":"+host.Port, conf)
	if err != nil {
		logError("Не удалось подключиться: ", err, host.IP)
		return err
	}
	defer connection.Close()

	// Создание сессии
	session, err := connection.NewSession()
	if err != nil {
		logError("Не удалось создать сессию: ", err, host.IP)
		return err
	}
	defer session.Close()

	// Переменная для хранения результата
	var splicedCommands string

	for i, command := range commands {
		// Добавляем элемент к результату
		splicedCommands += string(command)
		// Если это не последний элемент, добавляем разделитель
		if i < len(commands)-1 {
			splicedCommands += ";"
		}

	}

	// Выполнение команды на удаленном сервере
	output, err := session.CombinedOutput(splicedCommands)
	if err != nil {
		log.Printf("Не удалось выполнить команды: %v\n", err)
		//os.Exit(1)
		return err
	}

	fmt.Fprintf(buffer, string(output))

	// Пишем вывод напрямую в файл журнала для данной горутины
	fmt.Fprintf(logHost, buffer.String())

	outchan <- fmt.Sprintln(buffer.String())
	return nil

}

func containsOnlyWhitespaceOrEmpty(s string) bool {
	if len(strings.TrimSpace(s)) == 0 {
		return true // строка пуста
	}
	return false
}

func logError(s string, err error, host string) {
	log.Printf("****  ERROR  ERROR  ERROR ERROR  ERROR  ************************************ => \n  ")
	log.Printf("________________  HOST - %s  ____________________ \n\n", host)
	log.Printf("ERROR - %s  %v \n ", s, err)
	log.Printf(" <= ____  ERROR  ERROR  ERROR ERROR  ERROR  ____________________________________ \n \n ")

}
