# crocson

[howeyc/crocgui](https://github.com/howeyc/crocgui/releases/tag/v1.11.5)'in bir çatallaması (fork) olup, Android, Windows, Linux ve macOS için tasarlanmış [croc](https://github.com/schollz/croc) tabanlıdır.

<p align="center">
  <img src="images/phoneScreenshots/1.png?raw=true" width="200">
  <img src="images/phoneScreenshots/2.png?raw=true" width="200">
  <img src="images/phoneScreenshots/4.png?raw=true" width="200">
  <br>
</p>

<p align="center">
  <img src="../../Icon.png?raw=true" width="100"><br>
  <i>croc</i>'un yazarı <a href="https://github.com/schollz">Zack Scholl</a>, <i>croc</i> adını
  bir timsah taksicinin hayvanları güvenle taşımasına bir benzetme olarak seçmiştir.<BR>
  <i>crocson</i>, timsah taksicinin varisi olarak güvenli çevrimiçi iletişim sağlar.
</p>

## Dosya Aktarımı
`F-Droid: File Transfer` `FreeDesktop: Network;FileTransfer` `MS Store: Productivity` `Google Play: Communication;Tools`

- Bir aktarıcı (relay) üzerinden herhangi iki bilgisayar arasında dosya ve klasör aktarımı
- Birden fazla dosya ve klasör aktarımı (sıralı)
- Çapraz platform: Windows, Linux, macOS, Android
- Yerel sunucu veya port yönlendirmesi gerekmez
- IPv6 öncelikli, otomatik IPv4 geri dönüşü
- Kendi sunucunuzda aktarıcı barındırma (`crocson relay`)
- Özel aktarıcı bağlantı noktaları
- Dosya ve klasör seçim iletişim kutuları
- Sürükle-bırak (Windows, Linux, macOS)
- Komut satırı: bağımsız değişkenler ve stdin borusu (`cat file | crocson`)
- Android: Dosya yöneticilerindeki "Paylaş" menüsüyle bir veya birden fazla dosya gönderme
- Android: "Birlikte aç"
- Pano ile metin/URL gönderme
- Android: iç içe klasörler olmadan gönder, iç içe klasörlerle al — .zip olarak kaydet
- Genel ve dosya bazında ilerleme çubukları
- Aktarımı iptal et düğmesi
- Gönder ↔ Al geçiş düğmesi

## WebDAV
`F-Droid: Connectivity;Cloud Storage & File Sync` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Productivity`

- Yerleşik WebDAV sunucusu (HTTP/HTTPS)
- Otomatik imzalı TLS sertifikası (deterministik, yerel IP'lere dayalı)
- Web tarayıcısı üzerinden dosya gezintisi, yükleme (iletişim kutusu veya sürükle-bırak), silme (dizin listeleme)
- Arayüzde WebDAV dosya ağacı
- Ana makine/bağlantı noktası seçimi
- WebDAV üzerinden ses/video akışı oynatma
- Şifrelenmiş tünel üzerinden WebDAV yönlendirme (genel IP olmadan uzaktan erişilebilir):
  - röle [croc](https://github.com/schollz/croc/pull/1113) [fork](https://github.com/abakCroc/croc/v10)
  - röle [magic-wormhole](https://github.com/magic-wormhole/magic-wormhole) — şema `ws:` [fork](https://github.com/abakum/wormhole-william)
  - röle [webwormhole](https://github.com/saljam/webwormhole) — şema `https:` [fork](https://github.com/abakum/webwormhole)


## Görüntülü Arama
`F-Droid: Voice & Video Chat` `FreeDesktop: Network;VideoConference` `MS Store: Social` `Google Play: Communication`

- P2P görüntülü arama (yerleşik HTML sayfası)
- Ekran paylaşımı: tarayıcı sekmesi, uygulama penceresi veya tüm masaüstü
- Görüntülü arama odaları (oluşturma/katılma/bekleme/sonlandırma)
- WebSocket üzerinden gerçek zamanlı video+ses iletimi
- İkinci katılımcı bağlanmadan önce ayna önizlemesi
- Sunucu tarafında görüntülü arama kaydı (WebM/MP4)
- Çözücü (codec) ve çözünürlük seçimi

## Sohbet
`F-Droid: Messaging` `FreeDesktop: Network;Chat` `MS Store: Social` `Google Play: Communication`

- Yerleşik metin sohbeti
- Mesaj geçmişi
- Mesaj alındığında sohbeti otomatik açma
- Sohbete ekran görüntüsü ve video kaydı gönderme

## Video/Ses Kaydı
`F-Droid: Multimedia` `FreeDesktop: AudioVideo;Recorder` `MS Store: Photo + video` `Google Play: Video Players & Editors`

- Tarayıcıda web kamerası/mikrofon ile video+ses kaydı
- Zaman damgalı kayıt (YYYYMMDD_HHMMSS_mmm)
- Kayıtları sohbete yayımlama

## Web Kamerası
`F-Droid: Multimedia` `FreeDesktop: AudioVideo` `MS Store: Photo + video` `Google Play: Photography`

- Tarayıcı üzerinden web kamerası görüntüsü yakalama
- Görüntülü aramalardan ekran görüntüsü alma

## Güvenlik
`F-Droid: Security` `FreeDesktop: Security` `MS Store: Security` `Google Play: Tools`

- PAKE tabanlı uçtan uca şifreleme
- Zamanlayıcılı bir kerelik parolalar (TOTP)
- Giriş alanından veya CROC_SECRET ortam değişkeninden gizli anahtar
- Şifreleme için eliptik eğri seçimi
- Özetleme: imohash, md5, xxhash, highway
- Aktarıcı parolası
- Aktarıcı üzerinden şifrelenmiş tünel

## QR Kodu ve Derin Bağlantılar
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Kod ifadesiyle QR kodu oluşturma
- Kamera ile QR kodu tarama
- Birden fazla Android tarayıcı desteği (Xiaomi, Samsung, OPlus, BinaryEye, Lens, ZXing, Chrome, Via, Samsung Browser, Opera mini, Microsoft, Firefox)
- Derin Bağlantılar: `https://abakum.github.io/croc#...` (base64 ile kodlanmış ayarlar)
- Derin Bağlantılar: `davX:` / `webdavX:` WebDAV'ı açmak için

## Aktarıcı Profilleri
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Aktarıcı profilleri listesi (kaydet/yükle)
- Aktarıcılar arasında geçiş
- Arayüz üzerinden yerel aktarıcı yönetimi
- Özel adres, IPv6, bağlantı noktaları, parola
- Vekil sunucu desteği: SOCKS5 (Tor dahil), HTTP

## Aktarım Ayarları
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Yerel aktarımı devre dışı bırakma (yalnızca aktarıcı)
- Yalnızca yerel gönderenlere bağlanma
- .gitignore desteği
- Dosyaların üzerine yazma
- Sıkıştırma açık/kapalı
- Çoğullama (multiplexing) açık/kapalı
- Gönderirken klasörleri ZIP'leme
- Karşıya yükleme hız sınırlaması

## Arayüz
`F-Droid: Personalization` `FreeDesktop: Settings` `MS Store: Personalization` `Google Play: Personalization`

- Temalar: sistem, açık, gri, koyu, siyah
- Yazı tipi seçimi (gömülü + sistem yazı tipleri)
- Çok dilli (en-US, tr-TR, ja-JP, zh-CN, ru-RU)
- Logoyu gizleme
- Renkli aktarım günlüğü
- Android: `adb logcat -s croc` ile günlük kaydı

## Komut Satırı Kipi

En az bir dosya-dışı parametre verilirse, crocson croc CLI olarak çalışır:
- Yarım kalan aktarımları sürdürme (resuming)
- Borular (stdin/stdout): `cat file | crocson send`
- Metin gönderme: `crocson send --text "hello"`
- Mobil cihazlar için QR kodu
- Betikler için sessiz kip
- Kodu panoya kopyalama
- Klasörleri hariç tutma (`--exclude`)
- Süreç güvenliği için CROC_SECRET ortam değişkeni

---

## Uygulama Mağazalarına Göre Kategori Sıralaması

| Feature | F-Droid | FreeDesktop | MS Store | Google Play |
|---|---|---|---|---|
| File Transfer | **File Transfer** | Network;FileTransfer | Productivity | Communication |
| WebDAV | **Connectivity**; Cloud Storage & File Sync | Network | Productivity | Productivity |
| Video Calls | **Voice & Video Chat** | Network;VideoConference | Social | Communication |
| Chat | **Messaging** | Network;Chat | Social | Communication |
| Video/Audio Recording | **Multimedia** | AudioVideo;Recorder | Photo + video | Video Players & Editors |
| Webcam | **Multimedia** | AudioVideo | Photo + video | Photography |
| Security | **Security** | Security | Security | Tools |
| QR Code / Deep Links | **Connectivity** | Network | Productivity | Tools |
| Relay Profiles | **Connectivity** | Network | Productivity | Tools |
| Transfer Settings | **Connectivity** | Network | Productivity | Tools |
| Interface | **Personalization** | Settings | Personalization | Personalization |
