package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const Username = "admin"
const Password = "admin"

type Product struct {
	gorm.Model
	Code  string
	Price uint
}

type DataOpt struct {
	db *gorm.DB
}

type StaticInfo struct {
	InfoName  string `gorm:"unique;not null"`
	InfoValue string
}

type StringSet map[string]struct{}

func newStringSet(elements []string) StringSet {
	s := make(map[string]struct{})
	for _, element := range elements {
		s[element] = struct{}{}
	}
	return s
}

func (s StringSet) add(element string) {
	s[element] = struct{}{}
}

func (s StringSet) contains(element string) bool {
	_, exists := s[element]
	return exists
}

func (s StringSet) remove(element string) {
	delete(s, element)
}

func (s StringSet) array() []string {
	keys := []string{}
	for key := range s {
		keys = append(keys, key)
	}
	return keys
}

type SelectedSensor struct {
	Name       string `gorm:"unique;not null" json:"name"`
	Channel    string `json:"channel"`
	Type       string `json:"type"`
	UserName   string `json:"user_name"`
	RealSelect uint   `json:"real_select"`
}
type Nav struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Pos  uint   `json:"pos"`
	Name string `json:"name"`
	Url  string `json:"url"`
	Icon string `json:"icon"`
}

//user
//password
//unselected_nets
//cpu_temp_name
//mb_temp_name

//selected_sensors    name: "nct6798-isa-0290fan1", channel: "input", type: "fan", user_name: "fan_1", real_selected: true

//navs                name: 'gitee', url: 'https://gitee.com', icon: 'https://gitee.com/favicon.ico'

func init_dataopt() DataOpt {
	log.Print("DataOpt init")
	do := DataOpt{}
	var err error
	do.db, err = gorm.Open(sqlite.Open("/data/database.db"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic("failed to connect database")
	}
	if !do.db.Migrator().HasTable(&StaticInfo{}) {
		log.Print("init static_infos")
		do.db.AutoMigrate(&StaticInfo{})
		do.db.Create(&StaticInfo{InfoName: "username", InfoValue: "admin"})
		password, _ := argon2id.CreateHash("admin", argon2id.DefaultParams)
		do.db.Create(&StaticInfo{InfoName: "password", InfoValue: password})
		do.db.Create(&StaticInfo{InfoName: "jwt_secret", InfoValue: uuid.New().String()})
		do.db.Create(&StaticInfo{InfoName: "cpu_temp_name"})
		do.db.Create(&StaticInfo{InfoName: "mb_temp_name"})
		do.db.Create(&StaticInfo{InfoName: "unselected_nets"})
	}
	if !do.db.Migrator().HasTable(&SelectedSensor{}) {
		log.Print("init selected_sensors")
		do.db.AutoMigrate(&SelectedSensor{})
		// do.db.Create(&SelectedSensor{Name: "nct6798-isa-0290fan1", Channel: "input", Type: "fan", UserName: "fan_1", RealSelect: true})
		// do.db.Create(&SelectedSensor{Name: "nct6798-isa-0290fan2", Channel: "input", Type: "fan", UserName: "fan_2", RealSelect: true})

	}
	if !do.db.Migrator().HasTable(&Nav{}) {
		log.Print("init navs")
		do.db.AutoMigrate(&Nav{})
		// do.db.Create(&Nav{Name: "迅雷", Pos: 1, Url: "http://192.168.2.247:2345", Icon: "https://xfile2.a.88cdn.com/file/k/avatar/default"})
		// do.db.Create(&Nav{Name: "gitee", Pos: 2, Url: "https://gitee.com", Icon: "https://gitee.com/favicon.ico"})
		// do.db.Create(&Nav{Name: "kimi", Pos: 3, Url: "https://kimi.moonshot.cn/", Icon: "https://statics.moonshot.cn/kimi-chat/favicon.ico"})

	}
	return do
}

// Create----------------------------------------------------------------
func (do *DataOpt) add_unselected_net(c *gin.Context) {
	input := gin.H{"name": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	net := input["name"].(string)
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "unselected_nets")
	unselected_nets_string := info.InfoValue
	unselected_nets := newStringSet(strings.Split(strings.Trim(unselected_nets_string, ","), ","))
	if unselected_nets.contains(net) {
		c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
		return
	}
	unselected_nets.add(net)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "unselected_nets").Update("info_value", strings.Join(unselected_nets.array(), ","))
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
}

func (do *DataOpt) add_selected_sensor(c *gin.Context) {
	var sensor SelectedSensor
	if err := c.ShouldBindJSON(&sensor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	do.db.Create(&sensor)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
}

func (do *DataOpt) add_nav(c *gin.Context) {
	var nav Nav
	if err := c.ShouldBindJSON(&nav); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	var navs []Nav
	result := do.db.Find(&navs)
	nav.Pos = uint(result.RowsAffected) + 1
	do.db.Select("Pos", "Name", "Url", "Icon").Create(&nav)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Added successfully"})
}

// Read  ----------------------------------------------------------------
func (do *DataOpt) jwt_secret() string {
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "jwt_secret")
	return info.InfoValue
}
func (do *DataOpt) username(c *gin.Context) {
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "username")
	username := info.InfoValue
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": username})
}
func (do *DataOpt) cpu_and_mem_temp(c *gin.Context) {
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "cpu_temp_name")
	cpu_temp_name := info.InfoValue
	do.db.First(&info, "info_name = ?", "mb_temp_name")
	mb_temp_name := info.InfoValue
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": gin.H{"cpu_temp_name": cpu_temp_name, "mb_temp_name": mb_temp_name}})
}
func (do *DataOpt) unselected_nets(c *gin.Context) {
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "unselected_nets")
	unselected_nets_string := info.InfoValue
	unselected_nets := strings.Split(strings.Trim(unselected_nets_string, ","), ",")
	if len(strings.Trim(unselected_nets_string, ",")) == 0 {
		c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": unselected_nets})
}
func (do *DataOpt) selected_sensors(c *gin.Context) {
	var selected_sensors []SelectedSensor
	result := do.db.Find(&selected_sensors)
	if result.Error != nil {
		log.Print(result.Error)
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": result.Error})
	}
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": selected_sensors})
}
func (do *DataOpt) navs(c *gin.Context) {
	var navs []Nav
	result := do.db.Order("pos asc").Find(&navs)
	if result.Error != nil {
		log.Print(result.Error)
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": result.Error})
	}
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "", "data": navs})
}

// Update----------------------------------------------------------------
func (do *DataOpt) change_username(c *gin.Context) {
	input := gin.H{"username": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	new_name := input["username"].(string)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "username").Update("info_value", new_name)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) change_password(c *gin.Context) {
	input := gin.H{"new": "", "old": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	encoded_new_password := input["new"].(string)
	encoded_old_password := input["old"].(string)
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "password")
	password := info.InfoValue
	decoded_new_password := rsaDecode(encoded_new_password)
	decoded_old_password := rsaDecode(encoded_old_password)
	match, err := argon2id.ComparePasswordAndHash(decoded_old_password, password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	if !match {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": "Wrong password"})
		return
	}
	new_password, _ := argon2id.CreateHash(decoded_new_password, argon2id.DefaultParams)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "password").Update("info_value", new_password)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) change_cpu_and_mb_temp(c *gin.Context) {
	input := gin.H{"cpu": "", "mb": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	cpu := input["cpu"].(string)
	mb := input["mb"].(string)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "cpu_temp_name").Update("info_value", cpu)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "mb_temp_name").Update("info_value", mb)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) change_selected_sensor(c *gin.Context) {
	var sensor SelectedSensor
	if err := c.ShouldBindJSON(&sensor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	do.db.Model(&SelectedSensor{}).Where("name = ?", sensor.Name).Select("channel", "type", "user_name", "real_select").Updates(sensor)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) change_unselected_nets(c *gin.Context) {
	input := gin.H{"unselected_nets": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	unselected_nets := input["unselected_nets"].(string)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "unselected_nets").Update("info_value", unselected_nets)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) change_nav(c *gin.Context) {
	var nav Nav
	if err := c.ShouldBindJSON(&nav); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	do.db.Model(&Nav{}).Where("id = ?", nav.ID).Updates(nav)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

func (do *DataOpt) switch_nav(c *gin.Context) {
	input := gin.H{"id": 0, "to": 0}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	var nav Nav
	do.db.First(&nav, uint(input["id"].(float64)))
	to := uint(input["to"].(float64))
	from := nav.Pos
	if from > to {
		do.db.Model(&Nav{}).Where("pos >= ? AND pos < ?", to, from).Update("pos", gorm.Expr("pos + 1"))
	}
	if from < to {
		do.db.Model(&Nav{}).Where("pos > ? AND pos <= ?", from, to).Update("pos", gorm.Expr("pos - 1"))
	}
	do.db.Model(&nav).Update("pos", to)
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Modified successfully"})
}

// Delete----------------------------------------------------------------
func (do *DataOpt) delete_unselected_net(c *gin.Context) {
	input := gin.H{"name": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	net := input["name"].(string)
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "unselected_nets")
	unselected_nets_string := info.InfoValue
	unselected_nets := newStringSet(strings.Split(strings.Trim(unselected_nets_string, ","), ","))
	if !unselected_nets.contains(net) {
		c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
		return
	}
	unselected_nets.remove(net)
	do.db.Model(&StaticInfo{}).Where("info_name = ?", "unselected_nets").Update("info_value", strings.Join(unselected_nets.array(), ","))
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
}

func (do *DataOpt) delete_selected_sensor(c *gin.Context) {
	input := gin.H{"name": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	sensor := input["name"].(string)
	do.db.Where("name = ?", sensor).Delete(&SelectedSensor{})
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
}

func (do *DataOpt) delete_nav(c *gin.Context) {
	input := gin.H{"id": 0}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	id := uint(input["id"].(float64))
	var nav Nav
	do.db.First(&nav, id)
	do.db.Model(&Nav{}).Where("pos >= ?", nav.Pos).Update("pos", gorm.Expr("pos - 1"))
	do.db.Delete(&Nav{ID: id})
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": "Deleted successfully"})
}

// func ss() {

// 	// 迁移 schema
// 	db.AutoMigrate(&Product{})

// 	// Create
// 	db.Create(&Product{Code: "D42", Price: 100})

// 	// Read
// 	var product Product
// 	db.First(&product, 1)                 // 根据整型主键查找
// 	db.First(&product, "code = ?", "D42") // 查找 code 字段值为 D42 的记录

// 	// Update - 将 product 的 price 更新为 200
// 	db.Model(&product).Update("Price", 200)
// 	// Update - 更新多个字段
// 	db.Model(&product).Updates(Product{Price: 200, Code: "F42"}) // 仅更新非零值字段
// 	db.Model(&product).Updates(map[string]interface{}{"Price": 200, "Code": "F42"})

// 	// Delete - 删除 product
// 	db.Delete(&product, 1)
// }
