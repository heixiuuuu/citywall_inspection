// Command seed 预置演示数据：测试账号、城墙区段与巡检点位。
// 幂等：users 表已有数据时跳过，不重复创建。
// 用法：go run ./cmd/seed -config ../conf/server.json
package main

import (
	"flag"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/database"
	"citywall/server/internal/module/identity"
	"citywall/server/internal/module/site"
)

func main() {
	configPath := flag.String("config", "conf/server.json", "path to server config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := database.Open(cfg.Database)
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}

	var count int64
	if err := db.Model(&identity.User{}).Count(&count).Error; err != nil {
		log.Fatalf("count users: %v", err)
	}
	if count > 0 {
		log.Print("seed: users already exist, skip")
		return
	}

	seed(db)
	log.Print("seed: done")
}

func seed(db *gorm.DB) {
	hash := func(pwd string) string {
		b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("hash password: %v", err)
		}
		return string(b)
	}

	// 测试账号（密码仅限 dev 演示）。
	admin := identity.User{
		Username:     strPtr("admin"),
		PasswordHash: hash("Admin123"),
		WechatOpenid: strPtr("wx_test_admin"),
		DisplayName:  "系统管理员",
		Status:       "ACTIVE",
	}
	reviewer := identity.User{
		Username:     strPtr("reviewer"),
		PasswordHash: hash("Review123"),
		WechatOpenid: strPtr("wx_test_reviewer"),
		DisplayName:  "审核员·演示",
		Status:       "ACTIVE",
	}
	inspector := identity.User{
		Username:     strPtr("inspector"),
		PasswordHash: hash("Insp123"),
		WechatOpenid: strPtr("wx_test_inspector"),
		DisplayName:  "巡检员·演示",
		Status:       "ACTIVE",
	}
	if err := db.Create(&admin).Error; err != nil {
		log.Fatalf("create admin: %v", err)
	}
	if err := db.Create(&reviewer).Error; err != nil {
		log.Fatalf("create reviewer: %v", err)
	}
	if err := db.Create(&inspector).Error; err != nil {
		log.Fatalf("create inspector: %v", err)
	}
	roles := []identity.UserRole{
		{UserID: admin.ID, RoleCode: identity.RoleAdmin},
		{UserID: admin.ID, RoleCode: identity.RoleReviewer},
		{UserID: reviewer.ID, RoleCode: identity.RoleReviewer},
		{UserID: inspector.ID, RoleCode: identity.RoleInspector},
	}
	if err := db.Create(&roles).Error; err != nil {
		log.Fatalf("create roles: %v", err)
	}

	// 台州府城墙示例区段与点位。
	segs := []site.WallSegment{
		{Code: "SEG_EAST", Name: "东城墙段", Description: "台州府城墙东段示例区段", Active: true},
		{Code: "SEG_SOUTH", Name: "南城墙段", Description: "台州府城墙南段示例区段", Active: true},
	}
	if err := db.Create(&segs).Error; err != nil {
		log.Fatalf("create segments: %v", err)
	}
	points := []site.InspectionPoint{
		{SegmentID: segs[0].ID, Code: "P-EAST-01", Name: "东城墙 1 号点", Latitude: 28.84563, Longitude: 121.13120, RadiusM: 50, LocationHint: "瓮城外墙东北角", Active: true},
		{SegmentID: segs[0].ID, Code: "P-EAST-02", Name: "东城墙 2 号点", Latitude: 28.84610, Longitude: 121.13205, RadiusM: 50, LocationHint: "敌台西侧", Active: true},
		{SegmentID: segs[1].ID, Code: "P-SOUTH-01", Name: "南城墙 1 号点", Latitude: 28.84220, Longitude: 121.12980, RadiusM: 50, LocationHint: "南门城楼东侧", Active: true},
	}
	if err := db.Create(&points).Error; err != nil {
		log.Fatalf("create points: %v", err)
	}
	log.Printf("seed: users=%d segments=%d points=%d", 3, len(segs), len(points))
}

func strPtr(s string) *string { return &s }
