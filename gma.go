package gma

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/unknown-gd/go-pack"

	"github.com/IGLOU-EU/go-wildcard/v2"
)

const IDENTIFIER = 0x44414D47
const VERSION = 3
const APP_ID = 4000
const COMPRESSION_SIGNATURE = 0xBEEFCACE

const CRC32_STEP = 4096
const HEADER_SIZE = 5

var ErrInvalidSignature = errors.New("invalid signature")
var ErrUnsupportedVersion = errors.New("unsupported version")
var ErrChecksumMismatch = errors.New("checksum mismatch")
var ErrIsDirectory = errors.New("file is a directory")

var crc32_buffer []byte = make([]byte, CRC32_STEP)

var CategoryList = []string{
	"gamemode",
	"map",
	"weapon",
	"vehicle",
	"npc",
	"entity",
	"tool",
	"effects",
	"model",
	"servercontent",
}

func CategoryExists(type_name string) bool {
	return slices.Contains(CategoryList, type_name)
}

var TagList = []string{
	"fun",
	"roleplay",
	"scenic",
	"movie",
	"realism",
	"cartoon",
	"water",
	"comic",
	"build",
}

func TagExists(tag_name string) bool {
	return slices.Contains(TagList, tag_name)
}

var BlackList = []string{
	"models/*.sw.vtx", // These variations are unused by the game
	"models/*.360.vtx",
	"models/*.xbox.vtx",

	"gamemodes/*/*/*.txt", // Only in the root gamemode folder please!
	"gamemodes/*/*/*.fgd",

	"gamemodes/*/content/models/*.sw.vtx",
	"gamemodes/*/content/models/*.360.vtx",
	"gamemodes/*/content/models/*.xbox.vtx",
}

var WhiteList = []string{
	"lua/*.lua",
	"scenes/*.vcd",
	"particles/*.pcf",
	"resource/fonts/*.ttf",
	"scripts/vehicles/*.txt",
	"resource/localization/*/*.properties",
	"maps/*.bsp",
	"maps/*.lmp",
	"maps/*.nav",
	"maps/*.ain",
	"maps/thumb/*.png",
	"sound/*.wav",
	"sound/*.mp3",
	"sound/*.ogg",
	"materials/*.vmt",
	"materials/*.vtf",
	"materials/*.png",
	"materials/*.jpg",
	"materials/*.jpeg",
	"materials/colorcorrection/*.raw",
	"models/*.mdl",
	"models/*.phy",
	"models/*.ani",
	"models/*.vvd",

	"models/*.vtx",

	"gamemodes/*/*.txt",
	"gamemodes/*/*.fgd",

	"gamemodes/*/logo.png",
	"gamemodes/*/icon24.png",
	"gamemodes/*/gamemode/*.lua",
	"gamemodes/*/entities/effects/*.lua",
	"gamemodes/*/entities/weapons/*.lua",
	"gamemodes/*/entities/entities/*.lua",
	"gamemodes/*/backgrounds/*.png",
	"gamemodes/*/backgrounds/*.jpg",
	"gamemodes/*/backgrounds/*.jpeg",
	"gamemodes/*/content/models/*.mdl",
	"gamemodes/*/content/models/*.phy",
	"gamemodes/*/content/models/*.ani",
	"gamemodes/*/content/models/*.vvd",

	"gamemodes/*/content/models/*.vtx",

	"gamemodes/*/content/materials/*.vmt",
	"gamemodes/*/content/materials/*.vtf",
	"gamemodes/*/content/materials/*.png",
	"gamemodes/*/content/materials/*.jpg",
	"gamemodes/*/content/materials/*.jpeg",
	"gamemodes/*/content/materials/colorcorrection/*.raw",
	"gamemodes/*/content/scenes/*.vcd",
	"gamemodes/*/content/particles/*.pcf",
	"gamemodes/*/content/resource/fonts/*.ttf",
	"gamemodes/*/content/scripts/vehicles/*.txt",
	"gamemodes/*/content/resource/localization/*/*.properties",
	"gamemodes/*/content/maps/*.bsp",
	"gamemodes/*/content/maps/*.nav",
	"gamemodes/*/content/maps/*.ain",
	"gamemodes/*/content/maps/thumb/*.png",
	"gamemodes/*/content/sound/*.wav",
	"gamemodes/*/content/sound/*.mp3",
	"gamemodes/*/content/sound/*.ogg",

	// static version of the data/ folder
	// (because you wouldn't be able to modify these)
	// We only allow filetypes here that are not already allowed above
	"data_static/*.txt",
	"data_static/*.dat",
	"data_static/*.json",
	"data_static/*.xml",
	"data_static/*.csv",

	"shaders/fxc/*.vcs",
}

func IsAllowedPath(file_path string) bool {
	for _, pattern := range BlackList {
		if wildcard.Match(pattern, file_path) {
			return false
		}
	}

	for _, pattern := range WhiteList {
		if wildcard.Match(pattern, file_path) {
			return true
		}
	}

	return false
}

type Header struct {
	Identifier uint32
	Version    uint8
}

var header_buffer []byte = make([]byte, HEADER_SIZE)

func (self *Header) Reset() {
	self.Identifier = IDENTIFIER
	self.Version = VERSION
}

func (self *Header) Read(reader io.ReadSeekCloser) error {
	_, err := reader.Read(header_buffer)
	if err == nil {
		self.Identifier = binary.LittleEndian.Uint32(header_buffer[:4])
		self.Version = header_buffer[HEADER_SIZE-1]
	} else {
		return err
	}

	return nil
}

func (self *Header) Write(writer io.WriteSeeker) error {
	binary.LittleEndian.PutUint32(header_buffer[:4], self.Identifier)
	header_buffer[HEADER_SIZE-1] = self.Version
	_, err := writer.Write(header_buffer)
	return err
}

type Description struct {
	Title    string   `json:"title"`
	Category string   `json:"type"`
	Tags     []string `json:"tags"`
	Ignore   []string `json:"ignore"`
	content  []byte
}

func (self *Description) Reset() {
	self.Title = "unknown"
	self.Category = ""
	self.Tags = []string{}
	self.Ignore = []string{
		// gmad specifiic files
		"addon.json",

		// Windows specifiic files
		"*thumbs.db",
		"*desktop.ini",

		// Git files
		".git*",

		// MacOS specifiic files
		"*/.DS_Store",
		".DS_Store",
	}

	self.content = []byte{}
}

func (self *Description) Read() error {
	err := json.Unmarshal(self.content, &self)
	if err != nil {
		return err
	}

	category := strings.ToLower(self.Category)
	tags := self.Tags

	for i := range tags {
		tags[i] = strings.ToLower(tags[i])
	}

	self.Category = category
	self.Tags = tags
	return nil
}

func (self *Description) ToJSON() ([]byte, error) {
	return json.Marshal(self)
}

func (self *Description) Write() error {
	data, err := self.ToJSON()
	if err != nil {
		return err
	}

	self.content = data
	return nil
}

type Metadata struct {
	SteamID   uint64
	Timestamp int64

	Title           string
	Description     Description
	RequiredContent []string

	Author  string
	Version int32
}

func (self *Metadata) Reset() {
	self.SteamID = 0
	self.Timestamp = time.Now().Unix()

	self.Title = "unknown"

	description := Description{}
	description.Reset()
	self.Description = description

	self.RequiredContent = []string{}

	self.Author = "unknown"
	self.Version = 1
}

func (self *Metadata) Read(addon *Addon, reader io.ReadSeekCloser) error {
	steam_id, err := pack.ReadUInt64(reader, false)
	if err == nil {
		self.SteamID = steam_id
	} else {
		return err
	}

	timestamp, err := pack.ReadUInt64(reader, false)
	if err == nil {
		self.Timestamp = int64(timestamp)
	} else {
		return err
	}

	if addon.Header.Version > 1 {
		required_list := []string{}

		for {
			str, str_length, err := pack.ReadNullTerminatedString(reader)
			if err != nil {
				return err
			}

			if str_length == 0 {
				break
			} else {
				required_list = append(required_list, str)
			}
		}

		self.RequiredContent = required_list
	}

	title, _, err := pack.ReadNullTerminatedString(reader)
	if err == nil {
		self.Title = title
	} else {
		return err
	}

	content, _, err := pack.ReadNullTerminatedBytes(reader)
	if err == nil {
		description := Description{
			Title:    "unknown",
			Category: "unknown",

			content: content,
		}

		self.Description = description
		description.Read()
	} else {
		return err
	}

	author, _, err := pack.ReadNullTerminatedString(reader)
	if err == nil {
		self.Author = author
	} else {
		return err
	}

	version, err := pack.ReadInt32(reader, false)
	if err == nil {
		self.Version = version
	} else {
		return err
	}

	return nil
}

func (self *Metadata) Write(addon *Addon, writer io.WriteSeeker) error {
	err := pack.WriteUInt64(writer, self.SteamID, false) // SteamID
	if err != nil {
		return err
	}

	err = pack.WriteUInt64(writer, uint64(self.Timestamp), false) // Timestamp
	if err != nil {
		return err
	}

	required_content := self.RequiredContent

	for i := range required_content {
		err = pack.WriteNullTerminatedString(writer, required_content[i]) // Required content
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt8(writer, 0) // Null terminator
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, self.Title) // Title
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedBytes(writer, self.Description.content, nil) // Description
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, self.Author) // Author
	if err != nil {
		return err
	}

	err = pack.WriteInt32(writer, self.Version, false) // Version
	if err != nil {
		return err
	}

	return nil
}

type File struct {
	Index    uint32
	Path     string
	Size     int64
	Checksum uint32

	DataPosition int64
	DataLocation string
}

func (self *File) ReadInfo(reader io.ReadSeekCloser, file_location string, file_offset int64) (bool, error) {
	index, err := pack.ReadUInt32(reader, false) // File index
	if err != nil {
		return false, err
	} else if index == 0 {
		return false, nil
	}

	path, _, err := pack.ReadNullTerminatedString(reader) // File path
	if err != nil {
		return false, err
	}

	size, err := pack.ReadInt64(reader, false) // File size
	if err != nil {
		return false, err
	}

	checksum, err := pack.ReadUInt32(reader, false) // File checksum
	if err != nil {
		return false, err
	}

	self.Index = index
	self.Path = path
	self.Size = size
	self.Checksum = checksum

	self.DataPosition = file_offset
	self.DataLocation = file_location

	return true, err
}

func (self *File) WriteInfo(writer io.WriteSeeker) error {
	err := pack.WriteUInt32(writer, self.Index, false) // Index
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, self.Path) // Path
	if err != nil {
		return err
	}

	err = pack.WriteInt64(writer, self.Size, false) // Size
	if err != nil {
		return err
	}

	return pack.WriteUInt32(writer, self.Checksum, false) // Checksum (CRC32)
}

func (self *File) ReadData() ([]byte, error) {
	file, err := os.Open(self.DataLocation)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(self.DataPosition, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, self.Size)

	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	defer file.Close()
	return data, nil
}

func (self *File) WriteData(writer io.WriteSeeker) ([]byte, error) {
	file, err := os.Open(self.DataLocation)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(self.DataPosition, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, self.Size)

	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (self *File) CalculateChecksum() (uint32, error) {
	file, err := os.Open(self.DataLocation)
	if err != nil {
		return 0, err
	}

	_, err = file.Seek(self.DataPosition, io.SeekStart)
	if err != nil {
		return 0, err
	}

	checksum := crc32.NewIEEE()
	file_size := self.Size

	end_position := int((file_size/CRC32_STEP)-1) * CRC32_STEP

	for i := 0; i <= end_position; i += CRC32_STEP {
		_, err := file.Read(crc32_buffer)
		if err != nil {
			return 0, err
		}

		checksum.Write(crc32_buffer)
	}

	remainder := file_size % CRC32_STEP

	if remainder != 0 {
		_, err := file.Read(crc32_buffer[:remainder])
		if err != nil {
			return 0, err
		}

		checksum.Write(crc32_buffer[:remainder])
	}

	return checksum.Sum32(), nil
}

func (self *File) UpdateChecksum() error {
	checksum, err := self.CalculateChecksum()
	if err == nil {
		self.Checksum = checksum
		return nil
	} else {
		return err
	}
}

type Addon struct {
	Header   Header
	Metadata Metadata

	Files []File

	Size     int64
	Checksum uint32

	Location string
}

func (self *Addon) Reset() {
	header := Header{}
	header.Reset()

	self.Header = header

	metadata := Metadata{}
	metadata.Reset()

	self.Metadata = metadata

	self.Files = []File{}

	self.Size = 0
	self.Checksum = 0
}

func (self *Addon) AddFile(file_path string, internal_path string) error {
	info, err := os.Stat(file_path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return ErrIsDirectory
	}

	file := File{
		Index:    uint32(len(self.Files) + 1),
		Path:     internal_path,
		Size:     info.Size(),
		Checksum: 0,

		DataPosition: 0,
		DataLocation: file_path,
	}

	file.UpdateChecksum()

	self.Files = append(self.Files, file)

	return nil
}

func (self *Addon) RemoveFile(index int) {
	if index >= 0 && index < len(self.Files) {
		self.Files = append(self.Files[:index], self.Files[index+1:]...)
	}
}

func (self *Addon) UpdateSize() int64 {
	var size int64 = HEADER_SIZE // Header

	metadata := self.Metadata

	size += 8 // SteamID
	size += 8 // Timestamp

	size += int64(len(metadata.Title) + 1)               // Title
	size += int64(len(metadata.Description.content) + 1) // Description
	size += int64(len(metadata.Author) + 1)              // Author

	size += 4 // Version

	for _, str := range metadata.RequiredContent {
		size += int64(len(str) + 1) // Required content
	}

	size += 1 // Null terminator

	for _, file := range self.Files {
		size += 4                         // Index
		size += int64(len(file.Path) + 1) // Path
		size += 8                         // Size
		size += 4                         // Checksum
		size += file.Size                 // Data
	}

	size += 4 // Null terminator
	size += 4 // Checksum

	self.Size = size
	return size
}

func (self *Addon) GetFileByIndex(index int) *File {
	files := self.Files
	if index < 0 || index >= len(files) {
		return nil
	}

	return &files[index]
}

func (self *Addon) GetFileByPath(path string) *File {
	for _, file := range self.Files {
		if file.Path == path {
			return &file
		}
	}

	return nil
}

func (self *Addon) GetFileCount() int {
	return len(self.Files)
}

func (self *Addon) Read(reader io.ReadSeekCloser) error {
	// Header
	err := self.Header.Read(reader)
	if err != nil {
		return err
	}

	// Metadata
	err = self.Metadata.Read(self, reader)
	if err != nil {
		return err
	}

	// File list
	file_location := self.Location
	var file_offset int64 = 0
	file_list := []File{}

	for {
		file := File{}

		success, err := file.ReadInfo(reader, file_location, file_offset)
		if err != nil {
			return err
		} else if success {
			file_list = append(file_list, file)
			file_offset += file.Size
		} else {
			break
		}
	}

	data_position, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for i := 0; i < len(file_list); i++ {
		file_list[i].DataPosition += data_position
	}

	self.Files = file_list

	// Addon size
	file_size, err := reader.Seek(-4, io.SeekEnd)
	if err != nil {
		return err
	}

	self.Size = file_size

	// Addon checksum
	file_checksum, err := pack.ReadUInt32(reader, false)
	if err != nil {
		return err
	}

	self.Checksum = file_checksum
	return nil
}

// func build_files(base_path string, directory_path string, files []File) ([]File, error) {
// 	entries, err := os.ReadDir(directory_path)
// 	if err != nil {
// 		return files, err
// 	}

// 	for _, entry := range entries {
// 		if entry.IsDir() {
// 			files, err = build_files(base_path, directory_path+"/"+entry.Name(), files)
// 			if err != nil {
// 				return files, err
// 			}
// 		} else {
// 			info, err := entry.Info()
// 			if err != nil {
// 				return files, err
// 			}

// 			abs_path := directory_path + "/" + entry.Name()
// 			rel_path, _ := strings.CutPrefix(abs_path, base_path)

// 			files = append(files, File{
// 				Index:        uint32(len(files)),
// 				Path:         rel_path,
// 				Size:         info.Size(),
// 				Checksum:     0,
// 				DataPosition: 0,
// 				DataLocation: abs_path,
// 			})
// 		}
// 	}

// 	return files, nil
// }

func (self *Addon) Write(file_path string) error {
	writer, err := os.Create(file_path)
	if err != nil {
		return err
	}

	err = self.Header.Write(writer)
	if err != nil {
		return err
	}

	err = self.Metadata.Write(self, writer)
	if err != nil {
		return err
	}

	for _, file := range self.Files {
		err = file.WriteInfo(writer)
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt32(writer, 0, false)
	if err != nil {
		return err
	}

	for _, file := range self.Files {
		_, err = file.WriteData(writer)
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt32(writer, 0, false)
	if err != nil {
		return err
	}

	defer writer.Close()
	return nil
}

func Open(file_path string) (*Addon, error) {
	file, err := os.Open(file_path)
	if err != nil {
		return nil, err
	}

	addon := Addon{}
	addon.Reset()

	addon.Location = file_path

	addon.Read(file)

	defer file.Close()
	return &addon, nil
}

func CalculateChecksum(reader io.ReadSeekCloser) (uint32, error) {
	file_size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}

	checksum := crc32.NewIEEE()
	file_size -= 4 // checksum bytes (crc32)

	end_position := int((file_size/CRC32_STEP)-1) * CRC32_STEP

	for i := 0; i <= end_position; i += CRC32_STEP {
		_, err := reader.Read(crc32_buffer)
		if err != nil {
			return 0, err
		}

		checksum.Write(crc32_buffer)
	}

	remainder := file_size % CRC32_STEP

	if remainder != 0 {
		_, err := reader.Read(crc32_buffer[:remainder])
		if err != nil {
			return 0, err
		}

		checksum.Write(crc32_buffer[:remainder])
	}

	return checksum.Sum32(), nil
}
