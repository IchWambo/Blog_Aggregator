package config

import (
	"encoding/json"
	"log"
	"os"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	DB_URL            string `json:"db_url"`
	Current_User_Name string `json:"current_user_name"`
}

func Read() *Config {

	file_data, err := os.ReadFile("/home/wambo/" + configFileName)
	if err != nil {
		log.Fatal("error reading file", err)
	}

	var cfg *Config

	err = json.Unmarshal(file_data, &cfg)
	if err != nil {
		log.Fatal("error Unmarshaling data", err)
	}

	return cfg
}

func (c *Config) SetUser(current_user_name string) error {
	c.Current_User_Name = current_user_name

	data, err := json.Marshal(c)
	if err != nil {
		log.Fatal("error Marshaling data", err)
	}
	os.WriteFile("/home/wambo/"+configFileName, data, 0777)

	return nil
}
