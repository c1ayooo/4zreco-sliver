package handlers

import (
	"path/filepath"
	"testing"

	"4zreco/sliver/protobuf/sliverpb"
	"4zreco/sliver/server/core"
	"4zreco/sliver/server/db"
	"4zreco/sliver/server/db/models"
	"github.com/gofrs/uuid"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBeaconRegisterHandlerPropagatesCapabilities(t *testing.T) {
	// SLIVER_ROOT_DIR 隔离：即便惰性初始化在本测试内首次触发（db.Client 替换前
	// 已有路径调用过 Session() 等），也只会建到临时目录的库，不污染真实 ~/.sliver。
	t.Setenv("SLIVER_ROOT_DIR", t.TempDir())

	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "beacon-capabilities.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := testDB.AutoMigrate(&models.Beacon{}, &models.BeaconTask{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	originalDB := db.Client
	db.Client = testDB
	t.Cleanup(func() {
		db.Client = originalDB
		sqlDB, dbErr := testDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	beaconID, err := uuid.NewV4()
	if err != nil {
		t.Fatalf("generate beacon ID: %v", err)
	}
	registerData, err := proto.Marshal(&sliverpb.BeaconRegister{
		ID: beaconID.String(),
		Register: &sliverpb.Register{
			Uuid:         beaconID.String(),
			Capabilities: sliverpb.CapabilityBOFV1,
		},
	})
	if err != nil {
		t.Fatalf("marshal beacon register: %v", err)
	}

	beaconRegisterHandler(core.NewImplantConnection("test", "n/a"), registerData)

	beacon, err := db.BeaconByID(beaconID.String())
	if err != nil {
		t.Fatalf("load registered beacon: %v", err)
	}
	if beacon.Capabilities != sliverpb.CapabilityBOFV1 {
		t.Fatalf("expected Capabilities=%d, got %d", sliverpb.CapabilityBOFV1, beacon.Capabilities)
	}
	if beacon.ToProtobuf().Capabilities != sliverpb.CapabilityBOFV1 {
		t.Fatalf("expected protobuf Capabilities=%d, got %d", sliverpb.CapabilityBOFV1, beacon.ToProtobuf().Capabilities)
	}
}
