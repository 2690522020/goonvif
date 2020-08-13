package goonvif

import (
	"encoding/xml"
	"errors"
	"fmt"
	goonvif "goonvif/onvif"
	Device "goonvif/onvif/device"
	"goonvif/onvif/xsd"
	"time"

	"goonvif/onvif/PTZ"
	"goonvif/onvif/media"
	"goonvif/onvif/soap"
	"goonvif/onvif/xsd/onvif"
	"io/ioutil"
	"log"
	"net/http"
)

var ONVIFDevices map[string]*ONVIFDevice

type ONVIFDevice struct {
	ID                 string
	Code               string
	Address            string
	UserName           string
	Password           string
	SelectProfileToken onvif.ReferenceToken
	Client             ONVIFClient
	Channels           []ONVIFChannel
	MaxProfiles        int
}

type ONVIFClient interface {
	CallMethod(method interface{}) (*http.Response, error)
}

type ONVIFChannel struct {
	StreamUri string
	Profile   onvif.Profile
}

func NewONVIFClient(id, code, address, username, password string, edition *string) (*ONVIFDevice, error) {
	client, err := goonvif.NewDevice(address, edition)
	if err != nil {
		log.Fatal("ONVIFClient Init Error -> ", err.Error())
		return nil, err
	}
	client.Authenticate(username, password)
	device := ONVIFDevice{
		ID:       id,
		Address:  address,
		UserName: username,
		Password: password,
		Client:   client,
	}
	err = device.FindAllProfile()
	if err != nil {
		log.Fatal("ONVIFClient Init Error -> ", err.Error())
		return nil, err
	}
	device.SelectProfile(code)
	if ONVIFDevices == nil {
		ONVIFDevices = make(map[string]*ONVIFDevice)
	}
	ONVIFDevices[id] = &device
	return &device, nil
}

func GetProfiles(address, username, password string, edition *string) (error, []Profile) {
	client, err := goonvif.NewDevice(address, edition)
	if err != nil {
		log.Fatal("ONVIFClient Init Error -> ", err.Error())
		return err, nil
	}
	client.Authenticate(username, password)
	device := ONVIFDevice{
		ID:       "util.GetGuID()",
		Address:  address,
		UserName: username,
		Password: password,
		Client:   client,
	}
	return device.GetAllProfile()
}

func (d *ONVIFDevice) FindAllProfile() error {
	serviceCapabilities := media.GetServiceCapabilities{}
	capabilitiesResponse, err := d.Client.CallMethod(serviceCapabilities)
	if err != nil {
		log.Fatal("GetServiceCapabilities Error", err.Error())
		return err
	} else if capabilitiesResponse.StatusCode != 200 {
		body := readResponse(capabilitiesResponse)
		log.Fatal("GetServiceCapabilities Error", body)
	} else {
		gsmBody := soap.SoapMessage(readResponse(capabilitiesResponse)).Body()
		var GPC media.Capabilities
		err = xml.Unmarshal([]byte(gsmBody), &GPC)
		if err != nil {
			log.Fatal("GetServiceCapabilities Xml Unmarshal Error", err.Error())
			return err
		}
		d.MaxProfiles = GPC.ProfileCapabilities.MaximumNumberOfProfiles
	}
	mediaProfiles := media.GetProfiles{}
	mediaProfilesResponse, err := d.Client.CallMethod(mediaProfiles)
	if err != nil {
		log.Fatal("FindAllProfile Error", err.Error())
		return err
	} else if mediaProfilesResponse.StatusCode != 200 {
		body := readResponse(mediaProfilesResponse)
		log.Fatal("GetProfiles Error", body)
		return errors.New("GetProfiles Error (" + body + ")")
	} else {
		gsmBody := soap.SoapMessage(readResponse(mediaProfilesResponse)).Body()
		var GPR media.GetProfilesResponse
		err = xml.Unmarshal([]byte(gsmBody), &GPR)
		if err != nil {
			log.Fatal("FindAllProfile Xml Unmarshal Error", err.Error())
			return err
		}
		if GPR.Profiles == nil || len(GPR.Profiles) == 0 {
			log.Fatal(gsmBody)
			return errors.New("NotAuthorized")
		}
		for _, v := range GPR.Profiles {
			Transport := onvif.Transport{Protocol: "RTSP"}
			StreamSetup := onvif.StreamSetup{Stream: "RTP-Unicast", Transport: Transport}
			StreamUri := media.GetStreamUri{StreamSetup: StreamSetup, ProfileToken: v.Token}
			getStreamUriResponse, err := d.Client.CallMethod(StreamUri)
			if err != nil {
				log.Fatal("FindAllProfile Error", err.Error())
				break
			} else {
				streamUriResponseBody := soap.SoapMessage(readResponse(getStreamUriResponse)).Body()
				var streamUriResponse media.GetStreamUriResponse
				err = xml.Unmarshal([]byte(streamUriResponseBody), &streamUriResponse)
				if err != nil {
					log.Fatal("FindAllProfile Xml Unmarshal Error", err.Error())
					return err
				}
				rtspUri := AddPwdOnRTSPUri(string(streamUriResponse.MediaUri.Uri), d.UserName, d.Password)
				d.Channels = append(d.Channels, ONVIFChannel{
					StreamUri: rtspUri,
					Profile:   v,
				})
			}
		}
		return err
	}
}

func (d *ONVIFDevice) GetAllProfile() (error, []Profile) {
	serviceCapabilities := media.GetServiceCapabilities{}
	capabilitiesResponse, err := d.Client.CallMethod(serviceCapabilities)
	if err != nil {
		log.Fatal("GetServiceCapabilities Error", err.Error())
		return err, nil
	} else {
		if capabilitiesResponse.StatusCode == 200 {
			b := readResponse(capabilitiesResponse)
			gsmBody := soap.SoapMessage(b).Body()
			var GPC media.Capabilities
			err = xml.Unmarshal([]byte(gsmBody), &GPC)
			if err != nil {
				log.Fatal("GetAllProfile Xml Unmarshal Error", err.Error())
				return err, nil
			}
			d.MaxProfiles = GPC.ProfileCapabilities.MaximumNumberOfProfiles
		}
	}
	mediaProfiles := media.GetProfiles{}
	mediaProfilesResponse, err := d.Client.CallMethod(mediaProfiles)
	if err != nil {
		log.Fatal("GetAllProfile Error", err.Error())
		return err, nil
	} else if mediaProfilesResponse.StatusCode != 200 {
		body := readResponse(mediaProfilesResponse)
		log.Fatal("GetProfiles Error", body)
		return errors.New("GetProfiles Error (" + body + ")"), nil
	} else {
		gsmBody := soap.SoapMessage(readResponse(mediaProfilesResponse)).Body()
		var GPR media.GetProfilesResponse
		err = xml.Unmarshal([]byte(gsmBody), &GPR)
		if err != nil {
			log.Fatal("GetAllProfile Xml Unmarshal Error", err.Error())
			return err, nil
		}
		if GPR.Profiles == nil || len(GPR.Profiles) == 0 {
			log.Fatal(gsmBody)
			return errors.New("NotAuthorized"), nil
		}
		var data []Profile
		for _, v := range GPR.Profiles {
			data = append(data, Profile{
				Token: string(v.Token),
				Name:  string(v.Name),
			})
		}
		return err, data
	}
}

type Profile struct {
	Token, Name string
}

func (d *ONVIFDevice) SelectProfile(code string) {
	for _, v := range d.Channels {
		if fmt.Sprintf("%v", v.Profile.Token) == code {
			d.SelectProfileToken = v.Profile.Token
			break
		}
	}
}

//xSpeed 水平-1-1 ySpeed 垂直 -1-1 zSpeed 放大缩小 -1-1
func (d *ONVIFDevice) PTZGoto(xSpeed float64, ySpeed float64, zSpeed float64) error {
	//ptz.GeoMove{
	//	XMLName:      "",
	//	ProfileToken: "",
	//	Target:       onvif.GeoLocation{},
	//	Speed:        onvif.PTZSpeed{},
	//	AreaHeight:   0,
	//	AreaWidth:    0,
	//}
	request := ptz.ContinuousMove{
		ProfileToken: d.SelectProfileToken,
		Velocity: onvif.PTZSpeed{
			PanTilt: onvif.Vector2D{
				X: xSpeed,
				Y: ySpeed,
				//Space: xsd.AnyURI("http://www.onvif.org/ver10/tptz/PanTiltSpaces/GenericSpeedSpace"),
			},
			Zoom: onvif.Vector1D{
				X: zSpeed,
				//Space: xsd.AnyURI("http://www.onvif.org/ver10/tptz/ZoomSpaces/ZoomGenericSpeedSpace"),
			},
		},
		//Timeout: xsd.Duration(time.Second * 10),
		// timeout not working
	}
	response, err := d.Client.CallMethod(request)
	if response != nil {
		_, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal("PTZGoto Response Read Failed:", err)
			return nil
		}
	}
	return err
}

func (d *ONVIFDevice) PTZStop() error {
	request := ptz.Stop{
		ProfileToken: d.SelectProfileToken,
		PanTilt:      true,
		Zoom:         true,
	}
	response, err := d.Client.CallMethod(request)
	if response != nil {
		_, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal("PTZStop Response Read Failed:", err)
			return nil
		}
	}
	return err
}

func (d *ONVIFDevice) GotoPreset(presetToken string) error {
	request := ptz.GotoPreset{
		ProfileToken: d.SelectProfileToken,
		PresetToken:  onvif.ReferenceToken(presetToken),
		Speed: onvif.PTZSpeed{
			PanTilt: onvif.Vector2D{
				X: 0.0,
				Y: 0.0,
				//Space: xsd.AnyURI("http://www.onvif.org/ver10/tptz/PanTiltSpaces/GenericSpeedSpace"),
			},
			Zoom: onvif.Vector1D{
				X: 0.0,
				//Space: xsd.AnyURI("http://www.onvif.org/ver10/tptz/ZoomSpaces/ZoomGenericSpeedSpace"),
			},
		},
	}
	response, err := d.Client.CallMethod(request)
	if response != nil {
		_, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal("GotoPreset Response Read Failed:", err)
			return nil
		}
	}
	return err
}

func (d *ONVIFDevice) SetPreset(presetToken string) error {
	request := ptz.SetPreset{
		ProfileToken: d.SelectProfileToken,
		PresetToken:  onvif.ReferenceToken(presetToken),
		PresetName:   xsd.String(presetToken),
	}
	response, err := d.Client.CallMethod(request)
	if response != nil {
		_, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal("SetPreset Response Read Failed:", err)
			return nil
		}
	}
	return err
}

func (d *ONVIFDevice) SyncTime() error {
	now := time.Now().UTC()
	SyncTime := Device.SetSystemDateAndTime{
		TimeZone: onvif.TimeZone{
			TZ: "UTC-8",
		},
		UTCDateTime: onvif.DateTime{
			Time: onvif.Time{
				Hour:   xsd.Int(now.Hour()),
				Minute: xsd.Int(now.Minute()),
				Second: xsd.Int(now.Second()),
			},
			Date: onvif.Date{
				Year:  xsd.Int(now.Year()),
				Month: xsd.Int(now.Month()),
				Day:   xsd.Int(now.Day()),
			},
		},
	}
	_, err := d.Client.CallMethod(SyncTime)
	if err != nil {
		log.Fatal("SyncTime Response Read Failed:", err)
		return err
	}
	return err
}

func readResponse(resp *http.Response) string {
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("readResponse Error", err.Error())
		return ""
	}
	return string(b)
}

func AddPwdOnRTSPUri(rtsp string, username string, password string) string {
	if len(rtsp) > 7 {
		newuri := "rtsp://" + username + ":" + password + "@" + rtsp[7:]
		return newuri
	} else {
		return ""
	}
}

//func (this *ONVIFDevice) SetHomePosition() error {
//	setpreset := PTZ.SetPreset{
//		ProfileToken: this.SelectProfileToken,
//		PresetToken:  "1",
//	}
//	_, err := this.Client.CallMethod(setpreset)
//	if err != nil {
//		log.Println(err)
//	}
//	return err
//}
