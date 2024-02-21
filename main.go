package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"sync"
)

const ServersConfig = "./servers.conf"
const CommandsConfig = "./commands.conf"

var wg sync.WaitGroup

func main() {

	//recover and exit
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("\r\nFatal: %s \n\n", err)
			os.Exit(1)
		}
	}()

	// #####################################################################################################################

	var config config

	config.Servers.getServersFromConfig(ServersConfig)
	config.Commands.getCommandFromConfig(CommandsConfig)

	for _, host := range config.Servers {

		wg.Add(1)

		go runOnServer(host, config.Commands)

	}

	wg.Wait()

}

func runOnServer(host server, commands commands) error {

	fmt.Print("\n*************************************************************************\n\n")
	fmt.Printf("***  HOST - %s  ***\n\n", host.Host)

	// Загрузка закрытого ключа для аутентификации
	key, err := os.ReadFile(host.Keypath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось прочитать закрытый ключ: %v\n", err)
		return err
		//os.Exit(1)
	}

	// Создание Signer из закрытого ключа
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось разобрать закрытый ключ: %v\n", err)
		//os.Exit(1)
		return err
	}

	// Создание структуры конфигурации SSH
	conf := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Не использовать в продакшене!
	}

	// Подключение к серверу
	connection, err := ssh.Dial("tcp", host.Host, conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось подключиться: %v\n", err)
		//os.Exit(1)
		return err
	}
	defer connection.Close()

	// Создание сессии
	session, err := connection.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось создать сессию: %v\n", err)
		//os.Exit(1)
		return err
	}
	defer session.Close()

	// Переменная для хранения результата
	var result string

	for i, command := range commands {
		// Добавляем элемент к результату
		result += string(command)
		// Если это не последний элемент, добавляем разделитель
		if i < len(commands)-1 {
			result += ";"
		}

	}

	// Выполнение команды на удаленном сервере
	output, err := session.CombinedOutput(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось выполнить команды: %v\n", err)
		//os.Exit(1)
		return err
	}

	fmt.Println(string(output))

	// #####################################################################################################################
	fmt.Print("\n*************************************************************************")

	wg.Done()

	return nil

}
