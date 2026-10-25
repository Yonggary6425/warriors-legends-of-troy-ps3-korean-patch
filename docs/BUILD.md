# 빌드 방법

패치를 처음부터 다시 만드는 과정입니다. 게임 데이터와 번역 대본은 저장소에 없으므로, 정품 ISO에서 직접 추출해야 합니다.

## 필요한 것

- Python 3.11 이상
  - `pillow` (DDS DXT1/DXT5 저장 지원 버전), `numpy`, `scipy`, `fonttools`
  - 시험용 ISO를 만들 때만 `pycdlib`
- Go 1.22 이상 (패처 빌드)
- 글꼴 (Google Fonts 저장소, OFL)
  - `IBMPlexSansKR-SemiBold.ttf`, `IBMPlexSansKR-Regular.ttf`
  - `GowunDodum-Regular.ttf`
  - `NotoSerifKR[wght].ttf`

> 스크립트 안의 작업 경로는 제작 환경 기준으로 적혀 있습니다. 실행하기 전에 각 스크립트 상단의 경로를 자신의 환경에 맞게 고치세요.

## 도구 구성

```
tools/
├─ text/
│  ├─ extract.py      INI에서 키·원문을 뽑아 src.json 생성
│  ├─ qa.py           번역 검사 (누락, 태그 불일치, 줄바꿈 증가) 및 전체 매핑 생성
│  └─ build.py        번역을 INI로 다시 쓰기 (UTF-16BE, BOM, CRLF 유지)
├─ font/
│  ├─ g1n.py          G1N 읽기
│  ├─ hangul.py       글꼴별 한글 글리프 생성 (외곽선, 그림자, 입체 음영)
│  ├─ build_g1n.py    한글 글리프를 붙여 fonts.g1n 다시 만들기
│  ├─ render.py       원본 글리프 미리보기
│  └─ preview.py      게임 팔레트로 문장 합성 미리보기
├─ texture/
│  ├─ cpkg.py         cpkg/mpkg 풀기·묶기 (바뀐 청크만 재압축)
│  ├─ g1t.py          G1T 텍스처 해독
│  ├─ titletex.py     장 제목 텍스처
│  ├─ restyle.py      아틀라스 글자 조각 검출과 원본 색 추출 재작성
│  ├─ do_bg2.py       메인 메뉴·인물 이름표
│  ├─ diff.py         난이도 표시 (호 배치)
│  ├─ build_ui.py     LocalizedUIGameModePackage_ENG 빌드
│  └─ build_loc.py    LocalizedCommonPackage_ENG, CommonMasterPackage 빌드
└─ patcher/
   ├─ mkpatch/        차분 데이터 생성기 (Go)
   └─ apply/          패처 (Go, 차분 데이터 내장, ISO·폴더·UDF)
```

## 순서

### 1. 원본 추출
ISO를 7-Zip 등으로 풀고, 아래 파일을 작업 폴더로 복사합니다.

- `PS3_GAME/PARAM.SFO`
- `PS3_GAME/USRDIR/PS3/Localization/ENG/*.ini`
- `PS3_GAME/USRDIR/PS3/TrojanMusou/Fonts/fonts.g1n`
- `PS3_GAME/USRDIR/PS3/LocalizedCommonPackage_ENG.cpkg`
- `PS3_GAME/USRDIR/PS3/LocalizedUIGameModePackage_ENG.mpkg`
- `PS3_GAME/USRDIR/PS3/CommonMasterPackage.cpkg`

### 2. 텍스트
```
python tools/text/extract.py          # src.json, 번역 묶음 생성
# 번역 결과를 out/*.tsv (id<TAB>번역)로 작성
python tools/text/qa.py               # 검사, ko_full.json, charset.txt 생성
python tools/text/build.py            # 한국어 INI 생성
```
문장부호 ’ ‘ — …는 `{’}` `{‘}` `{—}` `{…}` 태그로 적습니다. `qa.py`가 태그 누락을 알려 줍니다.

### 3. 글꼴
```
python tools/font/build_g1n.py        # charset.txt 기준으로 fonts.g1n 생성
```

### 4. 텍스처
```
python tools/texture/build_ui.py      # LocalizedUIGameModePackage_ENG.mpkg
python tools/texture/build_loc.py     # LocalizedCommonPackage_ENG.cpkg, CommonMasterPackage.cpkg
```
배포용으로 묶을 때는 `cpkg.pack_reuse(새데이터, 원본파일)`를 써서, 바뀌지 않은 청크에 원본 압축 바이트를 그대로 씁니다.

### 5. 차분과 패처
```
cd tools/patcher/mkpatch && go build -o mkpatch .
./mkpatch <원본루트> <결과루트> list.txt patch.bin   # list.txt: 바뀐 파일 경로 목록
./mkpatch <v1.0.1루트> <결과루트> list_upd.txt patch_upd.bin   # 업데이트용
cp patch.bin patch_upd.bin ../apply/
cd ../apply
# 전체 패처 (patch.bin + patch_upd.bin 내장)
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o TroyKOR_Patcher.exe .
# 업데이트 패처 (patch.bin을 빈 파일로 두고 빌드)
: > patch.bin
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.edition=update" -o TroyKOR_Patcher_Update.exe .
```
`udf.go`는 ISO의 UDF 파일 목록을 읽고 고칩니다. `folder.go`는 게임 폴더 적용·백업·되돌리기를 담당합니다.
`mkpatch`는 만든 차분을 바로 되돌려 결과와 바이트 단위로 같은지 확인합니다.

### 6. 검증
- `pycdlib`로 원본 파일과 같은 구조(ISO9660 + Joliet, PS3 영역 표)의 시험용 ISO를 만듭니다.
- UDF 브리지 ISO(`genisoimage -udf`, UDF 2.50 메타데이터 파티션 변환본)에도 적용한 뒤 7-Zip으로 UDF·ISO9660 양쪽을 풀어 결과와 비교합니다.
- 패처를 적용한 뒤 두 디렉터리 트리에서 파일을 다시 꺼내 결과와 비교합니다.
- 마지막으로 실제 ISO에 적용하고 RPCS3에서 확인합니다.
