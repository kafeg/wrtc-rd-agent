package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/kafeg/wrtc-rd-agent/internal/encoders"
	"github.com/kafeg/wrtc-rd-agent/internal/rdisplay"
	"github.com/kafeg/wrtc-rd-agent/internal/rtc"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Most useful article describes WebRTC p2p magic
// http://forasoft.github.io/webrtc-in-plain-russian

const (
	defaultStunServer  = "stun:stun.l.google.com:19302"
	defaultInFileName  = "wrtcrd-in.dat"
	defaultOutFileName = "wrtcrd-out.dat"
)

type InputFileData struct {
	Offer  string `json:"offer"`
	Screen int    `json:"screen"`
}

type OutputFileData struct {
	Answer string `json:"answer"`
}

func readInputFile(filePath string) (InputFileData, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return InputFileData{}, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var result InputFileData
	err = json.Unmarshal([]byte(byteValue), &result)
	if err != nil {
		return InputFileData{}, err
	}

	return result, nil
}

func main() {
	// parse args
	stunServer := flag.String("stun", defaultStunServer, "STUN server URL (stun:)")
	inFileName := flag.String("in", defaultInFileName, "Input data filename (in:)")
	outFileName := flag.String("out", defaultOutFileName, "Output data filename (out:)")
	flag.Parse()

	fmt.Printf("Stun: %s, In: %s, Out: %s\n", *stunServer, *inFileName, *outFileName)
	fmt.Println("Init video provider and encoders")

	// setup major services
	var video rdisplay.ServiceInterface
	video, err := rdisplay.NewVideoProvider()
	if err != nil {
		log.Fatalf("Can't init video: %v", err)
	}
	_, err = video.Screens()
	if err != nil {
		log.Fatalf("Can't get screens: %v", err)
	}
	var enc encoders.ServiceInterface = &encoders.EncoderService{}
	if err != nil {
		log.Fatalf("Can't create encoder service: %v", err)
	}

	// parse input file
	fmt.Println("Read input data")
	inputData, err := readInputFile(*inFileName)
	if err != nil {
		log.Fatalf("Can't open input file: %v", err)
	}

	fmt.Println("Process WebRTC offer")
	var webrtc rtc.Service
	webrtc = rtc.NewRemoteScreenService(*stunServer, video, enc)

	//read offer and create answer
	peer, err := webrtc.CreateRemoteScreenConnection(inputData.Screen, 24)
	if err != nil {
		log.Fatalf("Can't create remote screen connection: %v", err)
	}

	// create request to STUN
	answer, err := peer.ProcessOffer(inputData.Offer)
	if err != nil {
		log.Fatalf("Can't process offer: %v", err)
	}

	fmt.Println("Write output data")
	outputData, err := json.Marshal(OutputFileData{
		Answer: answer,
	})
	if err != nil {
		log.Fatalf("Can't process offer: %v", err)
	}

	// write output file
	if *outFileName == "stdout" {
		log.Println(string(outputData))
	} else {
		_ = ioutil.WriteFile(*outFileName, outputData, 0644)
	}

	// here we already have webrtc which is trying to connect using our OFFER in IN file
	// and we need to send ANSWER back to the CALLER somehow

	fmt.Println("Waiting...")
	errors := make(chan error, 2)

	go func() {
		interrupt := make(chan os.Signal)
		signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
		errors <- fmt.Errorf("Received %v signal", <-interrupt)
	}()

	err = <-errors
	log.Printf("%s, exiting.", err)
}