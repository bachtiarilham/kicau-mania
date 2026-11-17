// Package model defines private engine settings and credential-free setup results.
package model

import (
	"net/url"
	"time"
)

type Channel struct {
	ID     string `json:"id"`
	IP     string `json:"ip,omitempty"`
	MAC    string `json:"mac,omitempty"`
	Source string `json:"source"`

	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	CredentialRef  string `json:"credential_ref,omitempty"`
	HasCredentials bool   `json:"has_credentials,omitempty"`
	Probe          *Probe `json:"probe,omitempty"`
}
type Configuration struct {
	Version  int       `json:"version" example:"1"`
	LANIP    string    `json:"lan_ip" example:"192.168.1.10"`
	LANCIDR  string    `json:"lan_cidr" example:"192.168.1.0/24"`
	NVRIP    string    `json:"nvr_ip,omitempty"`
	NVRMAC   string    `json:"nvr_mac,omitempty"`
	Channels []Channel `json:"channels"`
}
type Probe struct {
	VideoCodec string    `json:"video_codec"`
	AudioCodec string    `json:"audio_codec,omitempty"`
	HasAudio   bool      `json:"has_audio"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	FPS        float64   `json:"fps"`
	BFrames    int       `json:"b_frames"`
	TestedAt   time.Time `json:"tested_at"`
}

// ValidateMedia is shared by setup, startup and each capture reconnect.
func ValidateMedia(p Probe) error {
	if p.VideoCodec != "hevc" {
		return &Failure{Code: "UNSUPPORTED_VIDEO_CODEC", Message: "NVR harus menggunakan video HEVC/H.265; konversi codec tidak tersedia"}
	}
	return nil
}

type Network struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	CIDR string `json:"cidr"`
}
type Device struct {
	Endpoint string `json:"endpoint" example:"http://192.168.1.100/onvif/device_service"`
	Name     string `json:"name"`
}
type Profile struct {
	Token         string `json:"token"`
	Name          string `json:"name"`
	Source        string `json:"source"`
	VideoCodec    string `json:"video_codec"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	CredentialRef string `json:"credential_ref,omitempty"`
}
type DiscoveryRequest struct {
	IP       string `json:"ip" example:"192.168.1.10"`
	Endpoint string `json:"endpoint" example:"http://192.168.1.100/onvif/device_service"`
	Username string `json:"username"`
	Password string `json:"password"`
}
type JobRequest struct {
	Action string `json:"action" enums:"discover,profiles,probe,apply,recheck" example:"discover"`
	DiscoveryRequest
	Channel       Channel       `json:"channel"`
	Configuration Configuration `json:"configuration"`
}
type Job struct {
	Advice    string    `json:"advice,omitempty"`
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Progress  string    `json:"progress"`
	Result    any       `json:"result,omitempty"`
	Code      string    `json:"code,omitempty"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type RuntimeStatus struct {
	ChecksJobID    string        `json:"checks_job_id,omitempty"`
	SourceChecks   any           `json:"source_checks,omitempty"`
	CapacityNotice string        `json:"capacity_notice,omitempty"`
	Advice         string        `json:"advice,omitempty"`
	Channels       []ViewChannel `json:"channels"`
	Configured     bool          `json:"configured"`
	Ready          bool          `json:"ready"`
	ErrorCode      string        `json:"error_code,omitempty"`
	Session        any           `json:"session,omitempty"`
	Targets        []string      `json:"targets"`
	APIURLs        []string      `json:"api_urls"`
}
type SourceCheck struct {
	ID      string `json:"id"`
	Probe   *Probe `json:"probe,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Advice  string `json:"advice,omitempty"`
}
type ViewChannel struct {
	LiveVideoCodec string `json:"live_video_codec"`
	LiveAudioCodec string `json:"live_audio_codec,omitempty"`
	ID             string `json:"id"`
	HasAudio       bool   `json:"has_audio"`
	VideoCodec     string `json:"video_codec"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}
type Diagnostic struct {
	Advice    string `json:"advice,omitempty"`
	Component string `json:"component,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Failure contains only safe diagnostic text, never a source URI or process output.
type Failure struct{ Code, Message string }

func (e *Failure) Error() string { return e.Message }

func Advice(code string) string {
	switch code {
	case "MEDIA_UNAVAILABLE":
		return "MediaMTX belum berjalan. Periksa binary, port RTSP internal dan proses lain; API tetap tersedia."
	case "MAX_DURATION":
		return "Batas durasi sesi tercapai. Tunggu finalisasi sebelum memulai sesi berikutnya."
	case "DISK_LOW", "DISK_UNAVAILABLE":
		return "Periksa ruang disk dan izin folder rekaman; hentikan sesi dan tunggu finalisasi sebelum mengelola file."
	case "DISK_FULL":
		return "Stop sesi, periksa kapasitas disk dan upload; jangan menghapus manifest/segmen yang masih aktif."
	case "SOURCE_INTERRUPTED":
		return "Engine mencoba ulang channel setiap 5 detik. Periksa sumber/jaringan; channel lain tetap berjalan."
	case "BINARY_START_FAILED":
		return "Periksa lokasi binary, izin eksekusi dan batas proses pada komputer engine."
	case "RECORDING_FINALIZATION_FAILED":
		return "Simpan segmen untuk pemeriksaan dan lihat alasan validasi; jangan mengunggah file yang belum READY."
	case "SESSION_ACTIVE":
		return "Stop seluruh media dan tunggu status terminal sebelum mengubah konfigurasi."
	case "BINARY_MISSING", "ENCODER_MISSING":
		return "Periksa paket FFmpeg, ffprobe dan MediaMTX; codec sumber diteruskan tanpa encode ulang."
	case "DEVICE_AUTH_FAILED":
		return "Periksa akun ONVIF/RTSP serta sinkronisasi waktu PC dan NVR; kedua layanan dapat memakai akun berbeda."
	case "SOURCE_UNREACHABLE", "DEVICE_UNREACHABLE", "TIMEOUT":
		return "Periksa IP, port, path stream, kabel dan firewall; lalu jalankan tes koneksi kembali."
	case "DISCOVERY_BLOCKED", "NO_PROFILES", "DEVICE_PROTOCOL":
		return "Aktifkan ONVIF dan multicast UDP 3702 atau gunakan alamat layanan/RTSP manual."
	case "NETWORK_SCOPE", "NETWORK_UNAVAILABLE":
		return "Pilih interface LAN aktif pada subnet yang sama dengan perangkat."
	case "UNSUPPORTED_VIDEO_CODEC":
		return "Pilih profil NVR HEVC/H.265. Codec lain ditolak; konversi tidak tersedia."
	case "CONFIGURATION_UNAVAILABLE":
		return "Jalankan dengan akun Windows yang menyimpan konfigurasi DPAPI, atau masukkan ulang konfigurasi saat pindah PC."
	case "CONFIG_APPLY_FAILED", "ENGINE_START_FAILED":
		return "Periksa binary, port internal dan izin folder data; API tetap dapat dipakai untuk konfigurasi ulang."
	case "CANCELLED":
		return "Mulai pekerjaan baru jika pemeriksaan masih diperlukan."
	default:
		return "Periksa status engine dan hasil pemeriksaan aplikasi. Jangan membagikan kredensial atau log proses mentah."
	}
}

func Public(c Configuration) Configuration {
	c.Channels = append([]Channel(nil), c.Channels...)
	for i := range c.Channels {
		v := &c.Channels[i]
		u, e := url.Parse(v.Source)
		v.HasCredentials = v.Username != "" || v.Password != "" || e == nil && (u.User != nil || u.RawQuery != "")
		if e == nil {
			u.User = nil
			u.RawQuery = ""
			u.Fragment = ""
			v.Source = u.String()
		} else {
			v.Source = ""
		}
		v.Username, v.Password, v.CredentialRef = "", "", ""
	}
	return c
}
