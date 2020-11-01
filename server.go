package main

import (
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"net/http"
	// "errors"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Type uint8
type TagType uint8
type Proto uint16
type EtherType uint16

type Header struct {
	Version uint8
	Type    Type
	Proto   Proto
}

type Tag struct {
	Type   TagType
	Length uint8
	Data   []byte
}

type Packet struct {
	Header Header
	Tags   []Tag
	Data   []byte
}

type DatacollectorStruct struct {
	Type    string                     `json:"type"`
	Package DatacollectorPackageStruct `json:"package"`
}

type DatacollectorPackageStruct struct {
	DeviceID string `json:"device_id"`
	Data     string `json:"data"`
}

type ClearGrassMessageDataStruct struct {
	Battery      int    `json:"battery"`
	BatteryState string `json:"battery_state"`
	Co2          int    `json:"co2"`
	Co2State     string `json:"co2_state"`
	Co2Time      int    `json:"co2_time"`
	H            int    `json:"h"`
	HumiState    string `json:"humi_state"`
	Oh           int    `json:"oh"`
	Opm100       int    `json:"opm100"`
	Opm25        int    `json:"opm25"`
	Ot           int    `json:"ot"`
	Pm100        int    `json:"pm100"`
	Pm25         int    `json:"pm25"`
	Pm250        int    `json:"pm250"`
	Pm50         int    `json:"pm50"`
	PmState      string `json:"pm_state"`
	PmTime       int    `json:"pm_time"`
	T            int    `json:"t"`
	TempState    string `json:"temp_state"`
	TempUnit     string `json:"temp_unit"`
	Time         int    `json:"time"`
	Tvoc         int    `json:"tvoc"`
	TvocDuration int    `json:"tvoc_duration"`
	TvocState    string `json:"tvoc_state"`
	TvocTime     int    `json:"tvoc_time"`
	TvocUnit     string `json:"tvoc_unit"`
	Version      string `json:"version"`
	VersionType  string `json:"version_type"`
}

type ClearGrassMessageStruct struct {
	// Data      ClearGrassMessageDataStruct `json:"sensorData"`
	Data      json.RawMessage
	Mac       string `json:"mac"`
	Timestamp int    `json:"timestamp"`
	Type      string `json:"type"`
	Version   string `json:"version"`
}

type StatisticsStruct struct {
	Warnings          int
	IncomingProcessed int
	TZSPProcessed     int
	L2Processed       int
	L3Processed       int
	L4Processed       int
	L7Processed       int
	Started           time.Time
}

const (
	TagPadding            TagType = 0x00
	TagEnd                TagType = 0x01
	TagRawRSSI            TagType = 0x0a
	TagSNR                TagType = 0x0b
	TagDataRate           TagType = 0x0c
	TagTimestamp          TagType = 0xd
	TagContentionFree     TagType = 0x0f
	TagDecrypted          TagType = 0x10
	TagFCSError           TagType = 0x11
	TagRXChannel          TagType = 0x12
	TagPacketCount        TagType = 0x28
	TagRXFrameLength      TagType = 0x29
	TagWLANRadioHDRSerial TagType = 0x3c
)

var ApplicationDescription string = "TZSP Server"
var BuildVersion string = "0.0.0a"
var DeviceID int = 9000
var Debug bool = false
var DryRun bool = false

var TZSPheaderLength = 4
var TCPFlagACK = []byte{0x80, 0x10}
var TCPFlagPSHACK = []byte{0x80, 0x18}

var StatisticsNotifierPeriod int = 60
var Statistics = StatisticsStruct{
	Warnings:      0,
	TZSPProcessed: 0,
	L2Processed:   0,
	L3Processed:   0,
	L4Processed:   0,
	L7Processed:   0,
	Started:       time.Now(),
}

func handleSignal() {
	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChannel
		log.Print("SIGINT")
		duration := time.Now().Sub(Statistics.Started)
		log.Printf("Counters: TZSP: %d, L2: %d, L3: %d, L4: %d, L7: %d, duration: %v", Statistics.TZSPProcessed, Statistics.L2Processed, Statistics.L3Processed, Statistics.L4Processed, Statistics.L7Processed, duration)
		os.Exit(0)
	}()
}

func statisticsNotifier() {
	go func() {
		for {
			time.Sleep(60 * time.Second)
			duration := time.Now().Sub(Statistics.Started)
			log.Printf("Counters: TZSP: %d, L2: %d, L3: %d, L4: %d, L7: %d, duration: %v", Statistics.TZSPProcessed, Statistics.L2Processed, Statistics.L3Processed, Statistics.L4Processed, Statistics.L7Processed, duration)
		}
	}()
}

func main() {

	bindPtr := flag.String("bind", "127.0.0.1:37008", "Address and port to listen")
	enableDebugPtr := flag.Bool("debug", false, "Enable verbose output")
	enableDryRunPtr := flag.Bool("dry-run", false, "Dry run")
	showVersionPtr := flag.Bool("version", false, "Show version")

	flag.Parse()
	handleSignal()
	statisticsNotifier()

	if *enableDebugPtr {
		Debug = true
	}

	if *enableDryRunPtr {
		log.Print("This is a dry run")
		DryRun = true
	}

	if *showVersionPtr {
		fmt.Printf("%s\n", ApplicationDescription)
		fmt.Printf("Version: %s\n", BuildVersion)
		os.Exit(0)
	}

	listen_address, err := net.ResolveUDPAddr("udp4", *bindPtr)
	if err != nil {
		log.Fatal("Error while resolving address: ", err)
	}

	connection, err := net.ListenUDP("udp", listen_address)
	if err != nil {
		log.Fatal("Error while opening socket: ", err)
	}

	defer connection.Close()

	log.Print("Listening on ", listen_address.String())

	read_buffer := make([]byte, 65535)

	for {

		length, remote, err := connection.ReadFrom(read_buffer)
		if err != nil {
			panic(err)
		}

		go func() {

			log.Print("Accepted packet from ", remote, " length=", length)
			message := read_buffer[:length]

			Statistics.IncomingProcessed += 1

			if Debug {
				log.Printf("Dump:\n%s", hex.Dump(message))
			}

			if len(message) < TZSPheaderLength {
				log.Printf("Warning: Message is too short!\n")
			}

			handleTZSP(message)

		}()
	}

}

func handleTZSP(data []byte) {

	var tzspVersion = uint8(data[0])
	var tzspType = uint8(data[1])
	var tzspEncapsulatedProto = uint16(binary.BigEndian.Uint16(data[2:4]))

	Statistics.TZSPProcessed += 1

	if Debug {
		log.Print("TZSP Info: version=", tzspVersion, " type=", tzspType, " proto=", tzspEncapsulatedProto)
	}

	data = data[TZSPheaderLength:]
	tagEnd := 0

	for len(data) > 0 && tagEnd == 0 {

		log.Print("TZSP Tag: starting with data len: ", len(data), " tag end: ", tagEnd)

		var tagType = TagType(data[0])

		switch {

		case tagType == TagPadding:
			log.Print("TZSP Tag: TagPadding")

		case tagType == TagEnd:
			log.Print("TZSP Tag: TagEnd")
			tagEnd = 1

		default:
			log.Print("TZSP Tag: Skipping ", tagType)
		}

		if tagEnd == 0 {

			var tagLength = data[1]
			log.Print("TZSP Tag: Tag length ", tagLength)

			data = data[tagLength+2:]
			log.Print("TZSP Tag: Moving to ", tagLength, " data len: ", len(data), " tag end: ", tagEnd)
		}
	}

	switch {

	case tzspEncapsulatedProto == 1:
		log.Print("TZSP Encapsulated L2 protocol: Ethernet")
		handleEthernet(data)

	case tzspEncapsulatedProto == 18:
		log.Print("TZSP Encapsulated L2 protocol: IEEE 802.11")
		// log.Print("Warning: IEEE 802.11 is not supported.")
		handle80211(data)

	case tzspEncapsulatedProto == 119:
		log.Print("TZSP Encapsulated L2 protocol: Prism Header")
		log.Print("Warning: Prism Header is not supported.")

	case tzspEncapsulatedProto == 127:
		log.Print("TZSP Encapsulated L2 protocol: WLAN AVS")
		log.Print("Warning: WLAN AVS is not supported.")

	}
}

func handleMQTT(data []byte) {

	log.Print("L7 MQTT package length=", len(data))
	Statistics.L7Processed += 1

	if len(data) < 48 {
		log.Printf("Warning! MQTT payload length %s < 48. Which is too small.", len(data))
		log.Printf("Dump:\n%s", hex.Dump(data))
		return
	}

	header := data[:47]
	mqtt_len := header[2:4]
	mqtt_topic := string(header[4:42])
	mqtt_topic_parts := strings.Split(mqtt_topic, "/")

	if len(mqtt_topic_parts) < 2 {
		log.Printf("Error processing MQTT: topic unexpected: %s", mqtt_topic)
		log.Printf("Dump:\n%s", hex.Dump(data))
		return
	}

	mqtt_device_id := mqtt_topic_parts[2]

	log.Printf("L7 MQTT Message length: %0#4x", mqtt_len)
	log.Printf("L7 MQTT Topic: %s", mqtt_topic)

	payload := data[42:]

	if bytes.Equal(payload[(len(payload)-2):], []byte{0xc0, 0x00}) {
		log.Print("Warning: trimming {0xc0, 0x00} bytes from the end of the payload.")
		payload = payload[:(len(payload) - 2)]
	}

	var ClearGrassMessage = ClearGrassMessageStruct{}

	err := json.Unmarshal(payload, &ClearGrassMessage)
	if err != nil {
		log.Print("Error while reading JSON: ", err)
		log.Printf("Dump:\n%s", hex.Dump(payload))
		return
	}

	log.Print("JSON dump: ", ClearGrassMessage.Data)

	// 	log.Print("JSON CO2: ", ClearGrassMessage.Data.Co2)
	// 	log.Print("JSON Temp: ", ClearGrassMessage.Data.T)
	// 	log.Print("JSON PM2.5: ", ClearGrassMessage.Data.Pm250)
	// 	log.Print("JSON RH: ", ClearGrassMessage.Data.H)
	// 	log.Print("JSON tVOC: ", ClearGrassMessage.Data.Tvoc)
	// 	log.Print("JSON Bat: ", ClearGrassMessage.Data.Battery)

	metrics_tpl := `air_monitor,device_id=%s co2=%v
air_monitor,device_id=%s t=%v
air_monitor,device_id=%s pm250=%v
air_monitor,device_id=%s h=%v
air_monitor,device_id=%s tvoc=%v
air_monitor,device_id=%s battery=%v
`
	metrics := fmt.Sprintf(metrics_tpl,
		mqtt_device_id, "0", // ClearGrassMessage.Data.Co2,
		mqtt_device_id, "0", // ClearGrassMessage.Data.T,
		mqtt_device_id, "0", // ClearGrassMessage.Data.Pm250,
		mqtt_device_id, "0", // ClearGrassMessage.Data.H,
		mqtt_device_id, "0", // ClearGrassMessage.Data.Tvoc,
		mqtt_device_id, "0") // ClearGrassMessage.Data.Battery)

	if Debug {
		log.Printf("Metrics:\n%s", metrics)
	}

	if !DryRun {

		httpTransport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		client := &http.Client{Transport: httpTransport}
		request, _ := http.NewRequest("POST", "https://victoriametrics.marge.0123e.ru/write", bytes.NewBufferString(metrics))
		response, _ := client.Do(request)
		defer response.Body.Close()

		log.Print("VictoriaMetrics answered: ", response.Status)

		var DatacollectorPackage = DatacollectorPackageStruct{
			DeviceID: mqtt_device_id,
			Data:     string(payload),
		}

		var DatacollectorPayload = DatacollectorStruct{
			Type:    "air_monitor",
			Package: DatacollectorPackage,
		}

		var params bytes.Buffer
		encoder := gob.NewEncoder(&params)

		err = encoder.Encode(DatacollectorPayload)
		if err != nil {
			log.Fatal("encode error:", err)
		}

		encoded, err := json.Marshal(DatacollectorPayload)
		if err != nil {
			log.Fatal("encode error:", err)
		}

		client = &http.Client{}
		request, _ = http.NewRequest("POST", "http://api.datacollector.ru/v3/add/", bytes.NewReader(encoded))
		response, _ = client.Do(request)
		defer response.Body.Close()

		log.Print("Datacollector answered: ", response.Status)
	}
}

func handleUDP(data []byte) {
	log.Print("L4 UDP is not implemented yet")
}

func handleTCP(data []byte) {

	log.Print("L4 TCP package length=", len(data))
	Statistics.L4Processed += 1

	header := data[:32]
	src_port := binary.BigEndian.Uint16(header[0:2])
	dst_port := binary.BigEndian.Uint16(header[2:4])
	seq := header[4:8]
	ack := header[8:12]
	flags := header[12:14]
	log.Printf("L4 TCP Flags: % x", flags)

	switch {
	case bytes.Equal(flags, TCPFlagPSHACK):
		log.Print("L4 TCP Flags: PSH, ACK")

		if len(data) < 32 {
			log.Printf("Warning! TCP data length < 32 (%s), seq=% #x, ack=% #x", len(data), seq, ack)
			log.Printf("Dump:\n%s", hex.Dump(data))
			return
		}

		payload := data[32:]

		if len(payload) < 1 {
			log.Printf("Warning! TCP payload is empty, sport=%#x, dport=%#x seq=% #x, ack=% #x", src_port, dst_port, seq, ack)
			log.Printf("Dump:\n%s", hex.Dump(data))
			return
		}

		encapsulated_protocol := payload[0]
		log.Printf("L4 TCP Encapsulated protocol: %#2x", encapsulated_protocol)
		switch {

		case encapsulated_protocol == 0xc0 && dst_port == 1883:
			log.Print("L7 MQTT Ping Request")

		case encapsulated_protocol == 0x30 && dst_port == 1883:
			log.Print("L7 MQTT Publish Message")
			handleMQTT(payload)
		}
	default:
		log.Printf("L4 Unsupported TCP Flag: %0#4x\n", flags)
	}

}

func handleIPv4(data []byte) {

	if len(data) < 20 {
		log.Print("Warning: IP packet is too small")
	}

	log.Print("L3 IPv4 packet length=", len(data))

	Statistics.L3Processed += 1

	header := data[:20]

	tcp_version := make([]byte, 1)
	tcp_version = header[:1]
	log.Printf("L3 IP version: %#2x\n", tcp_version)

	total_length := header[3:4]
	log.Printf("L3 Total length: %#x\n", hex.EncodeToString(total_length))

	encapsulated_protocol := header[9]
	log.Printf("L3 IP Encapsulated L4 Protocol: %0#2x\n", encapsulated_protocol)

	var src_ip net.IP = header[12:16]
	var dst_ip net.IP = header[16:20]
	log.Print("L3 Source IPv4: ", src_ip, " Destination IPv4: ", dst_ip)

	switch {

	case encapsulated_protocol == 0x06:
		handleTCP(data[20:])

	default:
		log.Printf("L3 Unhandled protocol: %#2x\n", encapsulated_protocol)

	}
}

func handleEthernet(data []byte) {

	if len(data) < 14 {
		log.Print("Warning: Ethernet frame is too small")
		return
	}

	log.Print("L2 Ethernet frame length=", len(data))

	Statistics.L2Processed += 1

	eth_header := data[:14]
	eth_data := data[14:]

	var dst_mac net.HardwareAddr = eth_header[0:6]
	var src_mac net.HardwareAddr = eth_header[6:12]
	log.Print("L2 Source MAC: ", src_mac, " Destination MAC: ", dst_mac)

	var l3_protocol = EtherType(binary.BigEndian.Uint16(eth_header[12:14]))
	log.Printf("L2 Ethernet Encapsulated L3 Protocol: %0#4x\n", l3_protocol)

	switch {

	case l3_protocol == 0x0800:
		handleIPv4(eth_data)

	default:
		log.Print("Warning: Unsupported L3 protocol: ", l3_protocol)
		return
	}

}

func handle80211(data []byte) {

	if len(data) < 22 {
		log.Print("Warning: IEEE 802.11 frame is too small")
		return
	}

	log.Print("L2 IEEE 802.11 frame length=", len(data))

	Statistics.L2Processed += 1

	l2_header := data[4:22]
	// eth_data := data[14:]

	var ap_mac net.HardwareAddr = l2_header[0:6]
	var src_mac net.HardwareAddr = l2_header[6:12]
	var dst_mac net.HardwareAddr = l2_header[12:18]
	log.Print("L2 Source MAC: ", src_mac, " Destination MAC: ", dst_mac, " AP MAC: ", ap_mac)

	// var l3_protocol = EtherType(binary.BigEndian.Uint16(eth_header[12:14]))
	log.Printf("L2 IEEE 802.11 Encapsulated L3 Protocol: %0#4x\n", 0x00)

}

func handleConnection(connection net.UDPConn) {

	read_buffer := make([]byte, 65535)
	log.Print("Start")

	for {

		length, remote, err := connection.ReadFrom(read_buffer)
		if err != nil {
			log.Print("Error while handling reading from socket: ", err)
		}

		log.Print("Accepted packet from ", remote, " length=", length)

	}

	log.Print("Fin")

}
