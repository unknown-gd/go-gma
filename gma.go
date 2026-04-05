package gma

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
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

func (header *Header) Reset() {
	header.Identifier = IDENTIFIER
	header.Version = VERSION
}

func (header *Header) Read(reader io.ReadSeekCloser) error {
	_, err := reader.Read(header_buffer)
	if err == nil {
		header.Identifier = binary.LittleEndian.Uint32(header_buffer[:4])
		header.Version = header_buffer[HEADER_SIZE-1]
	} else {
		return err
	}

	return nil
}

func (header *Header) Write(writer io.WriteSeeker) error {
	binary.LittleEndian.PutUint32(header_buffer[:4], header.Identifier)
	header_buffer[HEADER_SIZE-1] = header.Version
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

func (description *Description) Reset() {
	description.Title = "unknown"
	description.Category = ""
	description.Tags = []string{}
	description.Ignore = []string{
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

	description.content = []byte{}
}

func (description *Description) Read() error {
	err := json.Unmarshal(description.content, &description)
	if err != nil {
		return err
	}

	category := strings.ToLower(description.Category)
	tags := description.Tags

	for i := range tags {
		tags[i] = strings.ToLower(tags[i])
	}

	description.Category = category
	description.Tags = tags
	return nil
}

func (description *Description) ToJSON() ([]byte, error) {
	return json.Marshal(description)
}

func (description *Description) Write() error {
	data, err := description.ToJSON()
	if err != nil {
		return err
	}

	description.content = data
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

func (metadata *Metadata) Reset() {
	metadata.SteamID = 0
	metadata.Timestamp = time.Now().Unix()

	metadata.Title = "unknown"

	description := Description{}
	description.Reset()
	metadata.Description = description

	metadata.RequiredContent = []string{}

	metadata.Author = "unknown"
	metadata.Version = 1
}

func (metadata *Metadata) Read(addon *Addon, reader io.ReadSeekCloser) error {
	steam_id, err := pack.ReadUInt64(reader, false)
	if err == nil {
		metadata.SteamID = steam_id
	} else {
		return err
	}

	timestamp, err := pack.ReadUInt64(reader, false)
	if err == nil {
		metadata.Timestamp = int64(timestamp)
	} else {
		return err
	}

	if addon.Header.Version > 1 {
		var required_list []string = []string{}

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

		metadata.RequiredContent = required_list
	}

	title, _, err := pack.ReadNullTerminatedString(reader)
	if err == nil {
		metadata.Title = title
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

		metadata.Description = description
		description.Read()
	} else {
		return err
	}

	author, _, err := pack.ReadNullTerminatedString(reader)
	if err == nil {
		metadata.Author = author
	} else {
		return err
	}

	version, err := pack.ReadInt32(reader, false)
	if err == nil {
		metadata.Version = version
	} else {
		return err
	}

	return nil
}

func (metadata *Metadata) Write(addon *Addon, writer io.WriteSeeker) error {
	err := pack.WriteUInt64(writer, metadata.SteamID, false) // SteamID
	if err != nil {
		return err
	}

	err = pack.WriteUInt64(writer, uint64(metadata.Timestamp), false) // Timestamp
	if err != nil {
		return err
	}

	required_content := metadata.RequiredContent

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

	err = pack.WriteNullTerminatedString(writer, metadata.Title) // Title
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedBytes(writer, metadata.Description.content, nil) // Description
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, metadata.Author) // Author
	if err != nil {
		return err
	}

	err = pack.WriteInt32(writer, metadata.Version, false) // Version
	if err != nil {
		return err
	}

	return nil
}

type File struct {
	Path string
	Size int64

	Checksum    uint32
	data_offset int64
}

func (file *File) Read(reader io.ReadSeekCloser, data_position int64) ([]byte, error) {
	_, err := reader.Seek(data_position+file.data_offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, file.Size)

	_, err = reader.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (file *File) WriteInfo(writer io.WriteSeeker, index uint32) error {
	err := pack.WriteUInt32(writer, index, false) // Index
	if err != nil {
		return err
	}

	err = pack.WriteNullTerminatedString(writer, file.Path) // Path
	if err != nil {
		return err
	}

	err = pack.WriteInt64(writer, file.Size, false) // Size
	if err != nil {
		return err
	}

	err = pack.WriteUInt32(writer, file.Checksum, false) // Checksum (CRC32)
	if err != nil {
		return err
	}

	return nil
}

func (file *File) WriteData(writer io.WriteSeeker, data []byte) error {
	_, err := writer.Write(data)
	return err
}

func (file *File) CalculateChecksum(reader io.ReadSeekCloser, data_position int64) (uint32, error) {
	_, err := reader.Seek(data_position+file.data_offset, io.SeekStart)
	if err != nil {
		return 0, err
	}

	checksum := crc32.NewIEEE()
	file_size := file.Size

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

type Addon struct {
	Header   Header
	Metadata Metadata

	Files         []File
	data_position int64

	Size     int64
	Checksum uint32
}

func (addon *Addon) Reset() {
	header := Header{}
	header.Reset()

	addon.Header = header

	metadata := Metadata{}
	metadata.Reset()

	addon.Metadata = metadata

	addon.Files = []File{}

	addon.data_position = 0

	addon.Size = 0
	addon.Checksum = 0
}

func (addon *Addon) UpdateSize() int64 {
	var size int64 = HEADER_SIZE // Header

	metadata := addon.Metadata

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

	for _, file := range addon.Files {
		size += 4                         // Index
		size += int64(len(file.Path) + 1) // Path
		size += 8                         // Size
		size += 4                         // Checksum
		size += file.Size                 // Data
	}

	size += 4 // Null terminator
	size += 4 // Checksum

	addon.Size = size
	return size
}

func ParseFiles(addon *Addon, reader io.ReadSeekCloser) error {
	var file_offset int64 = 0
	file_list := []File{}

	for {
		index, err := pack.ReadUInt32(reader, false)
		if err != nil {
			return err
		} else if index == 0 {
			break
		}

		path, _, err := pack.ReadNullTerminatedString(reader)
		if err != nil {
			return err
		}

		size, err := pack.ReadInt64(reader, false)
		if err != nil {
			return err
		}

		checksum, err := pack.ReadUInt32(reader, false)
		if err != nil {
			return err
		}

		file_list = append(file_list, File{
			Path:        path,
			Size:        size,
			Checksum:    checksum,
			data_offset: file_offset,
		})

		file_offset += size
	}

	data_position, err := reader.Seek(0, io.SeekCurrent)
	if err == nil {
		addon.data_position = data_position
	} else {
		return err
	}

	addon.Files = file_list
	return nil
}

func (addon *Addon) GetFileByIndex(index int) *File {
	files := addon.Files
	if index < 0 || index >= len(files) {
		return nil
	}

	return &files[index]
}

func (addon *Addon) GetFileByPath(path string) *File {
	for _, file := range addon.Files {
		if file.Path == path {
			return &file
		}
	}

	return nil
}

func (addon *Addon) GetFileCount() int {
	return len(addon.Files)
}

func (addon *Addon) Read(reader io.ReadSeekCloser) error {
	// Header
	err := addon.Header.Read(reader)
	if err != nil {
		return err
	}

	// Metadata
	err = addon.Metadata.Read(addon, reader)
	if err != nil {
		return err
	}

	// Files
	err = ParseFiles(addon, reader)
	if err != nil {
		return err
	}

	// Size
	file_size, err := reader.Seek(-4, io.SeekEnd)
	if err != nil {
		return err
	}

	addon.Size = file_size

	// Checksum
	file_checksum, err := pack.ReadUInt32(reader, false)
	if err != nil {
		return err
	}

	addon.Checksum = file_checksum
	return nil
}

func (addon *Addon) Write(writer io.WriteSeeker, files_content []string) error {
	err := addon.Header.Write(writer)
	if err != nil {
		return err
	}

	err = addon.Metadata.Write(addon, writer)
	if err != nil {
		return err
	}

	for index, file := range addon.Files {
		err = file.WriteInfo(writer, uint32(index))
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt32(writer, 0, false)
	if err != nil {
		return err
	}

	for _, file := range files_content {
		err = pack.WriteNullTerminatedString(writer, file)
		if err != nil {
			return err
		}
	}

	err = pack.WriteUInt32(writer, 0, false)
	if err != nil {
		return err
	}

	return nil
}

func Open(reader io.ReadSeekCloser) (*Addon, error) {
	addon := Addon{}
	addon.Reset()
	addon.Read(reader)
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
