package gma

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"gworx/pack"
	"hash/crc32"
	"io"
	"slices"
	"strings"

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

var crc32_buffer []byte = make([]byte, CRC32_STEP)

var TypeList = []string{
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

func TypeExists(type_name string) bool {
	return slices.Contains(TypeList, type_name)
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
	identifier uint32
	version    uint8
}

var header_buffer []byte = make([]byte, HEADER_SIZE)

func (header *Header) Read(reader io.ReadSeekCloser) error {
	_, err := reader.Read(header_buffer)
	if err == nil {
		header.identifier = binary.LittleEndian.Uint32(header_buffer[:4])
		header.version = header_buffer[HEADER_SIZE-1]
	} else {
		return err
	}

	return nil
}

type Description struct {
	Title    *string
	Category *string
	Tags     *[]string
	Ignore   *[]string
	Content  []byte
}

type DescriptionJSON struct {
	Title    string   `json:"title"`
	Category string   `json:"type"`
	Tags     []string `json:"tags"`
	Ignore   []string `json:"ignore"`
}

func (description *Description) Read() error {
	var json_schema DescriptionJSON

	err := json.Unmarshal(description.Content, &json_schema)
	if err != nil {
		return err
	}

	category := strings.ToLower(json_schema.Category)
	tags := json_schema.Tags

	for i := range tags {
		tags[i] = strings.ToLower(tags[i])
	}

	description.Title = &json_schema.Title
	description.Category = &category
	description.Tags = &tags
	description.Ignore = &json_schema.Ignore

	return nil
}

type Metadata struct {
	SteamID   uint64
	Timestamp uint64

	Title           string
	Description     Description
	RequiredContent []string

	Author  string
	Version int32
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
		metadata.Timestamp = timestamp
	} else {
		return err
	}

	if addon.Header.version > 1 {
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

	description, _, err := pack.ReadNullTerminatedBytes(reader)
	if err == nil {
		description := Description{Content: description}
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

	// println("Metadata:\n\tTitle: " + metadata.title + "\n\tDescription: " + metadata.description.content + "\n\tAuthor: " + metadata.author + "\n\tVersion: " + strconv.Itoa(int(metadata.version)))

	return nil
}

type File struct {
	addon *Addon

	Path string
	Size int64

	Checksum    uint32
	data_offset int64
}

func (file *File) Read(reader io.ReadSeekCloser) ([]byte, error) {
	_, err := reader.Seek(file.addon.data_position+file.data_offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, file.Size)

	_, err = reader.Read(bytes)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (file *File) CalculateChecksum(reader io.ReadSeekCloser) (uint32, error) {
	_, err := reader.Seek(file.addon.data_position+file.data_offset, io.SeekStart)
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

	Location string
	Size     int64
	Checksum uint32
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
			addon:       addon,
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

func Open(reader io.ReadSeekCloser, file_path string) (*Addon, error) {
	header := Header{
		identifier: IDENTIFIER,
		version:    VERSION,
	}

	metadata := Metadata{
		RequiredContent: []string{},
		SteamID:         0,
		Timestamp:       0,
		Title:           "",
		Description:     Description{Content: []byte{}},
		Author:          "",
		Version:         1,
	}

	addon := Addon{
		Header:   header,
		Metadata: metadata,
		Files:    []File{},
		Location: file_path,
	}

	err := header.Read(reader)
	if err != nil {
		return nil, err
	}

	err = metadata.Read(&addon, reader)
	if err != nil {
		return nil, err
	}

	err = ParseFiles(&addon, reader)
	if err != nil {
		return nil, err
	}

	file_size, err := reader.Seek(-4, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	addon.Size = file_size

	file_checksum, err := pack.ReadUInt32(reader, false)
	if err != nil {
		return nil, err
	}

	addon.Checksum = file_checksum

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
