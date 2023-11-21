package encoding

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"time"

	bmsapi "github.com/botnet-monitoring/grpc-api/gen"
)

func IPToWrapper(ip net.IP) *bmsapi.IPAddress {
	return OptionalIPToWrapper(&ip)
}

func OptionalIPToWrapper(ip *net.IP) *bmsapi.IPAddress {
	var wrapper *bmsapi.IPAddress

	if ip == nil {
		wrapper = nil
	} else {
		v4v6 := *ip
		v4 := v4v6.To4()
		if v4 != nil {
			var v4UInt32 uint32 = binary.BigEndian.Uint32(v4)
			wrapper = &bmsapi.IPAddress{
				Address: &bmsapi.IPAddress_V4{
					V4: &bmsapi.IPv4Address{
						Address: v4UInt32,
					},
				},
			}
		} else {
			var v6Bytes []byte = v4v6
			wrapper = &bmsapi.IPAddress{
				Address: &bmsapi.IPAddress_V6{
					V6: &bmsapi.IPv6Address{
						Address: v6Bytes,
					},
				},
			}
		}
	}

	return wrapper
}

func PortToWrapper(port uint16) *bmsapi.Port {
	return OptionalPortToWrapper(&port)
}

func OptionalPortToWrapper(port *uint16) *bmsapi.Port {
	var wrapper *bmsapi.Port

	if port == nil {
		wrapper = nil
	} else {
		wrapper = &bmsapi.Port{
			Port: uint32(*port),
		}
	}

	return wrapper
}

func StructToJSONWrapper(config interface{}) (*bmsapi.JSON, error) {
	var wrapper *bmsapi.JSON

	if config == nil {
		wrapper = nil
	} else {
		jsonString, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}

		wrapper = &bmsapi.JSON{
			JsonString: string(jsonString),
		}
	}

	return wrapper, nil
}

func TimeToWrapper(timestamp time.Time) *bmsapi.Timestamp {
	var wrapper *bmsapi.Timestamp

	seconds := timestamp.Unix()
	nanoSeconds := timestamp.UnixNano() - (timestamp.Unix() * 1000 * 1000 * 1000)

	wrapper = &bmsapi.Timestamp{
		UnixSeconds:     seconds,
		UnixNanoSeconds: nanoSeconds,
	}

	return wrapper
}

func OptionalTimestampToTime(timestamp *bmsapi.Timestamp) *time.Time {
	if timestamp == nil {
		return nil
	}

	t := time.Unix(timestamp.UnixSeconds, timestamp.UnixNanoSeconds)

	return &t
}

func StringToStringPtr(str string) *string {
	if str == "" {
		return nil
	} else {
		return &str
	}
}
