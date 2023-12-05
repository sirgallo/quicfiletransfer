package md5

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
)


//============================================= MD5


// CalculateMD5
//	Calculate MD5Checksum for the transferred file.
//	Return back the byte array representation.
func CalculateMD5(filePath string) ([]byte, error) {
	f, openErr := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if openErr != nil { return nil, openErr }

	_, seekErr := f.Seek(0, 0)
	if seekErr != nil { return nil, seekErr }

	hash := md5.New()
	_, generateMd5Err := io.Copy(hash, f)
	if generateMd5Err != nil { return nil, generateMd5Err }

	return hash.Sum(nil), nil
}

// DeserializeMD5ToHex
//	Tranform byte representation of MD5Checksum to hex.
func DeserializeMD5ToHex(input []byte) (string, error) {
	if len(input) != 16 { return "", errors.New("input length for md5sum should be 16") }
	return hex.EncodeToString(input), nil
}

// SerializeMD5ToBytes
//	Transform a string representation of MD5Checksum to byte array.
//	For transferring on the wire.
func SerializeMD5ToBytes(input string) ([]byte, error) {
	controlCharRegex := regexp.MustCompile(`[\x00-\x1F\x7F]`)
	md5 := controlCharRegex.ReplaceAllLiteralString(input, "")

	md5Bytes, decodeErr := hex.DecodeString(md5)
	if decodeErr != nil { return nil, decodeErr }

	return md5Bytes, nil
}

// ReadMD5FromFile
//	Read a MD5Checksum from a file.
//	The hex representation is then serialized to byte array.
func ReadMD5FromFile(md5FilePath string) ([]byte, error) {
	data, err := os.ReadFile(md5FilePath)
	if err != nil { return nil, err }

	md5Bytes, sErr := SerializeMD5ToBytes(string(data))
	if sErr != nil { return nil, sErr }
	if len(md5Bytes) != 16 { return nil, errors.New("md5 sum incorrect length") }
	
	return md5Bytes, nil
}