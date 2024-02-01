package util

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/mitchellh/mapstructure"
)

func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", errors.New("cannot find the client IP address")
}

func GetMacAddrs() (macAddrs []string) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return macAddrs
	}

	for _, netInterface := range netInterfaces {
		macAddr := netInterface.HardwareAddr.String()
		if len(macAddr) == 0 {
			continue
		}
		macAddrs = append(macAddrs, macAddr)
	}
	return macAddrs
}

func CallReflect(any any, name string, args ...any) []reflect.Value {
	inputs := make([]reflect.Value, len(args))
	for i := range args {
		inputs[i] = reflect.ValueOf(args[i])
	}

	if v := reflect.ValueOf(any).MethodByName(name); v.String() == "<invalid Value>" {
		return nil
	} else {
		return v.Call(inputs)
	}
}

func SetUnExportedStructField(ptr any, field string, newValue any) error {
	v := reflect.ValueOf(ptr).Elem().FieldByName(field)
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	nv := reflect.ValueOf(newValue)
	if v.Kind() != nv.Kind() {
		return fmt.Errorf("expected kind :%s, get kind: %s", v.Kind(), nv.Kind())
	}
	v.Set(nv)
	return nil
}

//func CopyStruct(source interface{}, dest interface{}) {
//	val := reflect.ValueOf(source)
//	destVal := reflect.ValueOf(dest).Elem()
//
//	for i := 0; i < val.NumField(); i++ {
//		field := val.Type().Field(i).Name
//		destField := destVal.FieldByName(field)
//		if destField.IsValid() && destField.CanSet() {
//			destField.Set(val.Field(i))
//		}
//	}
//}

func CopyStruct(src, dst interface{}) error {
	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Type().Field(i)
		dstField, found := dstVal.Type().FieldByName(srcField.Name)

		if found {
			if dstField.Type.AssignableTo(srcField.Type) {
				dstVal.FieldByName(srcField.Name).Set(srcVal.Field(i))
			} else {
				return fmt.Errorf("Cannot assign %s field", srcField.Name)
			}
		}
	}

	return nil
}

func ToTimeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(time.Time{}) {
			return data, nil
		}

		switch f.Kind() {
		case reflect.String:
			return time.Parse(time.RFC3339, data.(string))
		case reflect.Float64:
			return time.Unix(0, int64(data.(float64))*int64(time.Millisecond)), nil
		case reflect.Int64:
			return time.Unix(0, data.(int64)*int64(time.Millisecond)), nil
		default:
			return data, nil
		}
		// Convert it by parsing
	}
}

func decodeHook(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() == reflect.Int && t.Kind() == reflect.String {
		return strconv.Itoa(data.(int)), nil
	}
	return data, nil
}

func DecodeStruct(dst, src any) error {
	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &dst,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			ToTimeHookFunc()),
	})
	return decoder.Decode(src)
}

func ListFiles(path string) (res []string, err error) {
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			res = append(res, path)
		}
		return nil
	})
	return
}
