package encoders

/*
	Sliver Implant Framework
	Copyright (C) 2019  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"4zreco/var/sliver/protobuf/clientpb"
	"4zreco/var/sliver/server/assets"
	"4zreco/var/sliver/server/db"
	"4zreco/var/sliver/server/log"
	"4zreco/var/sliver/util"

	encutil "4zreco/var/sliver/util/encoders"
	"4zreco/var/sliver/util/encoders/traffic"
)

const (

	// EncoderModulus - The modulus used to calculate the encoder ID from a C2 request nonce
	// *** IMPORTANT *** ENCODER IDs MUST BE LESS THAN THE MODULUS
	EncoderModulus = uint64(65537)
	MaxN           = uint64(9999999)
)

var (
	encodersLog       = log.NamedLogger("encoders", "")
	trafficEncoderLog = log.NamedLogger("encoders", "traffic-encoders")

	TrafficEncoderFS = PassthroughEncoderFS{}

	Base64  = encutil.Base64{}
	Base58  = encutil.Base58{}
	Base32  = encutil.Base32{}
	Hex     = encutil.Hex{}
	English = encutil.English{}
	Gzip    = encutil.Gzip{}
	PNG     = encutil.PNGEncoder{}
	Nop     = encutil.NoEncoder{}

	NoEncoderID = uint64(0)

	// 以下 ID 与 UnavailableID 由 EnsureInitialized 在数据库/root dir 就绪后初始化。
	// 上游实现直接在包变量声明时读库/读资产，会让任何 import 本包的进程在 main()
	// 之前就要求 sliver 数据库与 root dir 可用；平台内嵌模式（internal/sliverhost）
	// 到启动期才设置两者，故改为显式初始化。零值在 EnsureInitialized 之前不可用于
	// 生成植入体（server 启动路径保证先调用）。
	Base64EncoderID  uint64
	Base58EncoderID  uint64
	Base32EncoderID  uint64
	HexEncoderID     uint64
	EnglishEncoderID uint64
	GzipEncoderID    uint64
	PNGEncoderID     uint64
	NopEncoderID     uint64
	UnavailableID    []uint64

	encodersInitOnce sync.Once
	encodersInitErr  error
)

// EnsureInitialized 初始化编码器 ID/映射/词表与流量编码器（幂等）。
// 必须在 sliver 数据库与 root dir 就绪后由 server 启动路径调用一次：
//   - 内嵌模式：internal/sliverhost.bootstrap
//   - 独立 server：server/cli 根命令（assets.Setup 之后）
func EnsureInitialized() error {
	encodersInitOnce.Do(func() {
		if Base64EncoderID, encodersInitErr = SetupDefaultEncoders("base64"); encodersInitErr != nil {
			return
		}
		if Base58EncoderID, encodersInitErr = SetupDefaultEncoders("base58"); encodersInitErr != nil {
			return
		}
		if Base32EncoderID, encodersInitErr = SetupDefaultEncoders("base32"); encodersInitErr != nil {
			return
		}
		if HexEncoderID, encodersInitErr = SetupDefaultEncoders("hex"); encodersInitErr != nil {
			return
		}
		if EnglishEncoderID, encodersInitErr = SetupDefaultEncoders("english"); encodersInitErr != nil {
			return
		}
		if GzipEncoderID, encodersInitErr = SetupDefaultEncoders("gzip"); encodersInitErr != nil {
			return
		}
		if PNGEncoderID, encodersInitErr = SetupDefaultEncoders("png"); encodersInitErr != nil {
			return
		}
		if NopEncoderID, encodersInitErr = SetupDefaultEncoders("nop"); encodersInitErr != nil {
			return
		}
		if UnavailableID, encodersInitErr = PopulateID(); encodersInitErr != nil {
			return
		}

		EncoderMap = map[uint64]encutil.Encoder{
			Base64EncoderID:  Base64,
			Base58EncoderID:  Base58,
			Base32EncoderID:  Base32,
			HexEncoderID:     Hex,
			EnglishEncoderID: English,
			GzipEncoderID:    Gzip,
			PNGEncoderID:     PNG,
		}
		FastEncoderMap = map[uint64]encutil.Encoder{
			Base64EncoderID: Base64,
			Base58EncoderID: Base58,
			Base32EncoderID: Base32,
			HexEncoderID:    Hex,
			GzipEncoderID:   Gzip,
		}

		encutil.SetEnglishDictionary(assets.English())
		TrafficEncoderFS = PassthroughEncoderFS{
			rootDir: filepath.Join(assets.GetRootAppDir(), "traffic-encoders"),
		}
		if err := loadTrafficEncodersFromFS(TrafficEncoderFS, func(msg string) {
			trafficEncoderLog.Debugf("[traffic-encoder] %s", msg)
		}); err != nil {
			trafficEncoderLog.Warnf("加载流量编码器失败: %s", err)
		}
	})
	return encodersInitErr
}

// SetupDefaultEncoders 读取或分配指定编码器的 ID（写入 sliver 数据库的 ResourceID）。
func SetupDefaultEncoders(name string) (uint64, error) {

	encoders, err := db.ResourceIDByType("encoder")
	if err != nil {
		encodersLog.Printf("Error:\n%s", err)
		return 0, err
	}

	for _, encoder := range encoders {
		if encoder.Name == name {
			return encoder.Value, nil
		}
	}

	id := GetRandomID()
	err = db.SaveResourceID(&clientpb.ResourceID{
		Type:  "encoder",
		Name:  name,
		Value: id,
	})
	if err != nil {
		encodersLog.Printf("Error:\n%s", err)
		return 0, err
	}

	return id, nil
}

// PopulateID 读取已占用的 ResourceID 值（用于生成不冲突的随机 ID）。
func PopulateID() ([]uint64, error) {
	// remove already used prime numbers from available pool
	resourceIDs, err := db.ResourceIDs()
	if err != nil {
		encodersLog.Printf("Error:\n%s", err)
		return nil, err
	}
	var unavailable []uint64
	for _, resourceID := range resourceIDs {
		unavailable = append(unavailable, resourceID.Value)
	}

	return unavailable, nil
}

// generate a random id and ensure it is not in use
func GetRandomID() uint64 {
	id := util.Intn(int(EncoderModulus))
	for slices.Contains(UnavailableID, uint64(id)) {
		id = util.Intn(int(EncoderModulus))
	}
	UnavailableID = append(UnavailableID, uint64(id))
	return uint64(id)
}

// EncoderMap - A map of all available encoders (native and traffic/wasm)
// 由 EnsureInitialized 填充（依赖运行时分配的编码器 ID）。
var EncoderMap = map[uint64]encutil.Encoder{}

// TrafficEncoderMap - Keeps track of the loaded traffic encoders (i.e., wasm-based encoder functions)
var TrafficEncoderMap = map[uint64]*traffic.TrafficEncoder{}

// FastEncoderMap - Keeps track of fast native encoders that can be used for large messages
// 由 EnsureInitialized 填充。
var FastEncoderMap = map[uint64]encutil.Encoder{}

// SaveTrafficEncoder - Save a traffic encoder to the filesystem
func SaveTrafficEncoder(name string, wasmBin []byte) error {
	if !strings.HasSuffix(name, ".wasm") {
		return fmt.Errorf("invalid encoder name, must end with .wasm")
	}
	wasmFilePath := filepath.Join(assets.GetTrafficEncoderDir(), filepath.Base(name))
	err := os.WriteFile(wasmFilePath, wasmBin, 0600)
	if err != nil {
		return err
	}
	return loadTrafficEncodersFromFS(TrafficEncoderFS, func(msg string) {
		trafficEncoderLog.Debugf("[traffic-encoder] %s", msg)
	})
}

// RemoveTrafficEncoder - Save a traffic encoder to the filesystem
func RemoveTrafficEncoder(name string) error {
	if !strings.HasSuffix(name, ".wasm") {
		return fmt.Errorf("invalid encoder name, must end with .wasm")
	}
	wasmFilePath := filepath.Join(assets.GetTrafficEncoderDir(), filepath.Base(name))
	info, err := os.Stat(wasmFilePath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to do
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		panic("wasmFilePath is a directory, this should never happen")
	}
	err = os.Remove(wasmFilePath)
	if err != nil {
		return err
	}
	return loadTrafficEncodersFromFS(TrafficEncoderFS, func(msg string) {
		trafficEncoderLog.Debugf("[traffic-encoder] %s", msg)
	})
}

// loadTrafficEncodersFromFS - Loads the wasm traffic encoders from the filesystem, for the
// server these will be loaded from: <app root>/traffic-encoders/*.wasm
func loadTrafficEncodersFromFS(encodersFS encutil.EncoderFS, logger func(string)) error {

	// Reset references pointing to traffic encoders
	for _, encoder := range TrafficEncoderMap {
		delete(EncoderMap, encoder.ID)
	}
	TrafficEncoderMap = map[uint64]*traffic.TrafficEncoder{}

	// Load WASM encoders
	encodersLog.Info("initializing traffic encoder map...")
	wasmEncoderFiles, err := encodersFS.ReadDir("traffic-encoders")
	if err != nil {
		return err
	}
	for _, wasmEncoderFile := range wasmEncoderFiles {
		encodersLog.Debugf("checking file: %s", wasmEncoderFile.Name())
		if wasmEncoderFile.IsDir() {
			continue
		}
		if !strings.HasSuffix(wasmEncoderFile.Name(), ".wasm") {
			continue
		}
		// WASM Module name should be equal to file name without the extension
		wasmEncoderModuleName := strings.TrimSuffix(wasmEncoderFile.Name(), ".wasm")
		wasmEncoderData, err := encodersFS.ReadFile(path.Join("traffic-encoders", wasmEncoderFile.Name()))
		if err != nil {
			encodersLog.Errorf("%s", fmt.Sprintf("failed to read file %s (%s)", wasmEncoderModuleName, err.Error()))
			return err
		}
		wasmEncoderID := traffic.CalculateWasmEncoderID(wasmEncoderData)
		trafficEncoder, err := traffic.CreateTrafficEncoder(wasmEncoderModuleName, wasmEncoderData, logger)
		if err != nil {
			encodersLog.Errorf("%s", fmt.Sprintf("failed to create traffic encoder from '%s': %s", wasmEncoderModuleName, err.Error()))
			return err
		}
		trafficEncoder.FileName = wasmEncoderFile.Name()
		if _, ok := EncoderMap[uint64(wasmEncoderID)]; ok {
			encodersLog.Errorf("%s", fmt.Sprintf("duplicate encoder id: %d", wasmEncoderID))
			return fmt.Errorf("duplicate encoder id: %d", wasmEncoderID)
		}
		EncoderMap[uint64(wasmEncoderID)] = trafficEncoder
		TrafficEncoderMap[uint64(wasmEncoderID)] = trafficEncoder
		encodersLog.Info(fmt.Sprintf("loading %s (id: %d, bytes: %d)", wasmEncoderModuleName, wasmEncoderID, len(wasmEncoderData)))
	}
	encodersLog.Info(fmt.Sprintf("loaded %d traffic encoders", len(wasmEncoderFiles)))
	return nil
}

// EncoderFromNonce - Convert a nonce into an encoder
func EncoderFromNonce(nonce uint64) (uint64, encutil.Encoder, error) {
	encoderID := uint64(nonce) % EncoderModulus
	if encoderID == 0 {
		return 0, new(encutil.NoEncoder), nil
	}
	if encoder, ok := EncoderMap[encoderID]; ok {
		return encoderID, encoder, nil
	}
	return 0, nil, fmt.Errorf("invalid encoder id: %d", encoderID)
}

// RandomEncoder - Get a random nonce identifier and a matching encoder
func RandomEncoder() (uint64, encutil.Encoder) {
	keys := make([]uint64, 0, len(EncoderMap))
	for k := range EncoderMap {
		keys = append(keys, k)
	}
	encoderID := keys[util.Intn(len(keys))]
	nonce := (randomUint64(MaxN) * EncoderModulus) + encoderID
	return nonce, EncoderMap[encoderID]
}

func randomUint64(max uint64) uint64 {
	buf := make([]byte, 8)
	rand.Read(buf)
	return binary.LittleEndian.Uint64(buf) % max
}

// PassthroughEncoderFS - Creates an encoder.EncoderFS object from a single local directory
type PassthroughEncoderFS struct {
	rootDir string
}

func (p PassthroughEncoderFS) Open(name string) (fs.File, error) {
	localPath := filepath.Join(p.rootDir, filepath.Base(name))
	if !strings.HasSuffix(localPath, ".wasm") {
		return nil, os.ErrNotExist
	}
	if stat, err := os.Stat(localPath); os.IsNotExist(err) || stat.IsDir() {
		return nil, os.ErrNotExist
	}
	return os.Open(localPath)
}

func (p PassthroughEncoderFS) ReadDir(_ string) ([]fs.DirEntry, error) {
	if _, err := os.Stat(p.rootDir); os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	ls, err := os.ReadDir(p.rootDir)
	if err != nil {
		return nil, err
	}
	var entries []fs.DirEntry
	for _, entry := range ls {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".wasm") {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (p PassthroughEncoderFS) ReadFile(name string) ([]byte, error) {
	localPath := filepath.Join(p.rootDir, filepath.Base(name))
	if !strings.HasSuffix(localPath, ".wasm") {
		return nil, os.ErrNotExist
	}
	if stat, err := os.Stat(localPath); os.IsNotExist(err) || stat.IsDir() {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(localPath)
}
