package gma

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ulikunitz/xz/lzma"
	"github.com/unknown-gd/go-pack"

	"github.com/IGLOU-EU/go-wildcard/v2"
)

const LZMA_SIGNATURE = 0x0000005D
const GMA_SIGNATURE = 0x44414D47
const GMA_VERSION = 3

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

func (self *Header) Reset() {
	self.Identifier = GMA_SIGNATURE
	self.Version = GMA_VERSION
}

var ErrInvalidSignature = errors.New("invalid signature")
var ErrUnsupportedVersion = errors.New("unsupported version")

func (self *Header) Read(reader io.ReadSeeker) error {
	var identifier uint32

	err := binary.Read(reader, binary.LittleEndian, &identifier)
	if err != nil {
		return err
	}

	self.Identifier = identifier

	if identifier != GMA_SIGNATURE {
		return ErrInvalidSignature
	}

	var version uint8

	err = binary.Read(reader, binary.LittleEndian, &version)
	if err != nil {
		return err
	}

	self.Version = version

	if version > GMA_VERSION {
		return ErrUnsupportedVersion
	}

	return nil
}

func (self *Header) Write(writer io.WriteSeeker) error {
	err := binary.Write(writer, binary.LittleEndian, &self.Identifier)
	if err != nil {
		return err
	}

	return binary.Write(writer, binary.LittleEndian, &self.Version)
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

	for i, tag := range tags {
		tags[i] = strings.ToLower(tag)
	}

	self.Category = category
	self.Tags = tags
	return nil
}

var ErrAddonLeastTag = errors.New("Addon must have at least one tag")
var ErrAddonMostTag = errors.New("Addon must have at most 3 tags")
var ErrAddonUniqueTag = errors.New("Addon must have unique tags")

func (self *Description) ToJSON() ([]byte, error) {
	data, err := json.Marshal(self)
	if err != nil {
		return nil, err
	}

	category := self.Category

	if !CategoryExists(category) {
		return nil, errors.New("Addon type '" + category + "' does not allowed")
	}

	tags := self.Tags
	tag_count := len(tags)

	if tag_count == 0 {
		return nil, ErrAddonLeastTag
	} else if tag_count > 3 {
		return nil, ErrAddonMostTag
	}

	for i := range tag_count {
		tag := tags[i]
		if !TagExists(tag) {
			return nil, errors.New("Addon tag '" + tag + "' does not allowed")
		}

		for j := range i {
			if tag == tags[j] {
				return nil, ErrAddonUniqueTag
			}
		}
	}

	return data, nil
}

func (self *Description) Update() error {
	data, err := self.ToJSON()
	if err != nil {
		return err
	}

	self.content = data
	return nil
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

func (self *Metadata) Read(addon *Addon, reader io.ReadSeeker) error {
	err := binary.Read(reader, binary.LittleEndian, &self.SteamID)
	if err != nil {
		return err
	}

	err = binary.Read(reader, binary.LittleEndian, &self.Timestamp)
	if err != nil {
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
			content:  content,
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

	return binary.Read(reader, binary.LittleEndian, &self.Version)
}

func (self *Metadata) Write(addon *Addon, writer io.WriteSeeker) error {
	err := binary.Write(writer, binary.LittleEndian, &self.SteamID) // SteamID
	if err != nil {
		return err
	}

	err = binary.Write(writer, binary.LittleEndian, &self.Timestamp) // Timestamp
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

	description := &self.Description
	description.Update()

	err = pack.WriteNullTerminatedBytes(writer, description.content, nil) // Description
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, self.Author) // Author
	if err != nil {
		return err
	}

	return binary.Write(writer, binary.LittleEndian, &self.Version) // Version
}

type File struct {
	Index    uint32
	Path     string
	Size     int64
	Checksum uint32

	DataPosition int64
	DataLocation string
}

func (self *File) ReadInfo(reader io.ReadSeeker, file_location string, file_offset int64) (bool, error) {
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

	defer file.Close()

	_, err = file.Seek(self.DataPosition, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, self.Size)

	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (self *File) WriteData(writer io.WriteSeeker) ([]byte, error) {
	file, err := os.Open(self.DataLocation)
	if err != nil {
		return nil, err
	}

	defer file.Close()

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

	defer file.Close()

	_, err = file.Seek(self.DataPosition, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return pack.CRC32IEEE(file, self.Size)
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

func (self *File) IsValid() (bool, error) {
	expected_checksum := self.Checksum
	if expected_checksum == 0 {
		return true, nil // No checksum
	}

	checksum, err := self.CalculateChecksum()
	if err != nil {
		return false, err
	}

	return checksum == expected_checksum, nil
}

type Addon struct {
	Header   Header
	Metadata Metadata

	Files []File

	Size     int64
	Checksum uint32

	Location string
	Legacy   bool
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
	self.Legacy = false
}

var ErrIsDirectory = errors.New("file is a directory")

func (self *Addon) AddFile(file_path string, internal_path string) error {
	if !IsAllowedPath(internal_path) {
		return errors.New("path '" + internal_path + "' is not allowed")
	}

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
	var size int64 = 5 // Header

	metadata := &self.Metadata

	size += 8 // SteamID
	size += 8 // Timestamp

	size += int64(len(metadata.Title) + 1) // Title

	description := metadata.Description
	description.Update()

	size += int64(len(description.content) + 1) // Description

	size += int64(len(metadata.Author) + 1) // Author

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

func (self *Addon) Read(reader io.ReadSeeker) error {
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

func (self *Addon) Write(file_path string) error {
	file, err := os.Create(file_path)
	if err != nil {
		return err
	}

	defer file.Close()

	// Header
	err = self.Header.Write(file)
	if err != nil {
		return err
	}

	// Metadata
	err = self.Metadata.Write(self, file)
	if err != nil {
		return err
	}

	// File list
	for _, f := range self.Files {
		err = f.WriteInfo(file)
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt32(file, 0, false)
	if err != nil {
		return err
	}

	// File data
	for _, f := range self.Files {
		_, err = f.WriteData(file)
		if err != nil {
			return err
		}
	}

	// Addon size
	file_size, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	self.Size = file_size

	// Addon checksum
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	checksum, err := pack.CRC32IEEE(file, file_size)
	if err != nil {
		return err
	}

	self.Checksum = checksum

	// Write checksum
	_, err = file.Seek(file_size, io.SeekStart)
	if err != nil {
		return err
	}

	err = pack.WriteUInt32(file, checksum, false)
	if err != nil {
		return err
	}

	return nil
}

func (self *Addon) CalculateChecksum() (uint32, error) {
	file, err := os.Open(self.Location)
	if err != nil {
		return 0, err
	}

	defer file.Close()

	file_size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}

	file_size -= 4 // checksum bytes (uint32/crc32)
	return pack.CRC32IEEE(file, file_size)
}

func (self *Addon) UpdateChecksum() error {
	checksum, err := self.CalculateChecksum()
	if err == nil {
		self.Checksum = checksum
		return nil
	} else {
		return err
	}
}

func (self *Addon) IsValid() (bool, error) {
	expected_checksum := self.Checksum
	if expected_checksum == 0 {
		return true, nil // No checksum
	}

	checksum, err := self.CalculateChecksum()
	if err != nil {
		return false, err
	}

	return checksum == expected_checksum, nil
}

func IsLegacy(reader io.ReadSeeker) (bool, error) {
	var signature uint32

	err := binary.Read(reader, binary.LittleEndian, &signature)
	if err != nil {
		return false, err
	}

	return signature == LZMA_SIGNATURE, nil
}

func Open(file_path string) (*Addon, error) {
	file, err := os.Open(file_path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	is_legacy, err := IsLegacy(file)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	addon := Addon{
		Legacy: is_legacy,
	}

	var reader *os.File

	// Legacy addon repack (lzma)
	if is_legacy {
		lzma_reader, err := lzma.NewReader(file)
		if err != nil {
			return nil, err
		}

		ext := filepath.Ext(file_path)
		file_path = file_path[:len(file_path)-len(ext)] + "_gworx.gma"
		addon.Location = file_path

		writer, err := os.Create(file_path)
		if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(lzma_reader)
		if err != nil {
			return nil, err
		}

		_, err = writer.Write(data)
		if err != nil {
			return nil, err
		}

		_, err = writer.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}

		reader = writer
	} else {
		addon.Location = file_path
		reader = file
	}

	err = addon.Read(reader)
	if err != nil {
		return nil, err
	}

	return &addon, nil
}
