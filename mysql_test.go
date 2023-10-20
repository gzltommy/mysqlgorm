package mysqlgorm

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"testing"
)

func TestMysql(t *testing.T) {
	mysqlI, err := NewMySQLClient(
		&SQLConfig{
			Host:     "127.0.0.1",
			User:     "metaverse",
			Password: "123456",
			Port:     3306,
			Database: "test",
		},
		&SSHConfig{
			Host: "192.168.1.2",
			User: "ubuntu",
			Port: 22,
			//KeyFile: "~/.ssh/id_rsa",
			KeyFile: "C:\\Users\\ww\\.ssh\\id_rsa",
			Type:    SshKeyTypeKey, // PASSWORD or KEY
		},
	)
	if err != nil {
		fmt.Printf("mysql connect error: %s", err.Error())
		return
	}

	u := User{}
	result := mysqlI.DB().Where("id = ?", 5131).First(&u) //自动生成 sql： SELECT * FROM `users`  WHERE (username = 'tommy') LIMIT 1
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		fmt.Println("找不到记录")
		return
	}
	//打印查询到的数据
	fmt.Println(u.UserName, u.Email)
}

type User struct {
	Id       int64  `gorm:"primary_key" json:"id"`
	UserName string `json:"user_name"`
	Bio      string `json:"bio"`
	Email    string `json:"email"`
}

func (u User) TableName() string {
	return "users"
}
