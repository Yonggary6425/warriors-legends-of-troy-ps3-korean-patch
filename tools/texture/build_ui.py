import sys,io; sys.path.insert(0,'/tmp/claude-0/work')
from restyle import *
from do_bg2 import build_bg2
from g1t import scan, decode
from cpkg import pack, unpack
def put(d,t,img):
  b=io.BytesIO(); img.save(b,'DDS',pixel_format={8:'DXT5',6:'DXT1'}[t['fmt']]); raw=b.getvalue()[128:]
  assert len(raw)==t['datalen'],(t['name'],len(raw),t['datalen'])
  d[t['dataoff']:t['dataoff']+t['datalen']]=raw
if __name__=='__main__':
  P='/mnt/user-data/uploads/Warriors Legends of Troy [U]/PS3_GAME/USRDIR/PS3/'
  d=bytearray(open('uimode.bin','rb').read()); T={t['name']:t for t in scan(bytes(d))}
  prev={}
  # Main_BG2
  im=decode(bytes(d),T['Main_BG2']); new=build_bg2(im); put(d,T['Main_BG2'],new); prev['Main_BG2']=new
  # Main_BG
  im=decode(bytes(d),T['Main_BG']).convert('RGBA'); src=im.copy()
  gold=src.crop((16,451,319,494))
  def meas(w,h,y0,y1):
    m=Image.new('RGBA',(w,h),(0,0,0,0)); m.paste(Image.new('RGBA',(w,y1-y0),(255,255,255,255)),(0,y0)); return m
  for b,t,al,ms in [((16,451,319,494),'도전','left',None),((13,511,322,559),'도전','left',None),((19,574,317,611),'도전','left',gold.crop((0,0,298,37))),((19,634,424,671),'장 선택','left',meas(405,37,4,33)),
                 ((195,703,318,735),'아이템','center',None),((182,768,332,801),'기술','center',None)]:
    replace(im,src,b,t,align=al,span=(b[0],b[0]+420) if al=='left' else (b[0]-40,b[2]+40),measure=ms)
  put(d,T['Main_BG'],im); prev['Main_BG']=im
  # Chap_Title
  im=decode(bytes(d),T['Chap_Title']).convert('RGBA'); src=im.copy()
  for b,t in [((73,6,441,40),'장 선택'),((123,70,391,103),'도전'),((41,134,473,168),'캐릭터 선택')]:
    replace(im,src,b,t,align='center',span=(20,492))
  put(d,T['Chap_Title'],im); prev['Chap_Title']=im
  # Extras_Title
  im=decode(bytes(d),T['Extras_Title']).convert('RGBA'); src=im.copy()
  for b,t in [((174,14,339,47),'부록'),((86,64,430,97),'영상관'),((107,113,405,146),'미술 전시관'),((93,164,418,197),'음악당')]:
    replace(im,src,b,t,align='center',span=(40,472))
  replace(im,src,(447,222,512,250),'신규',align='center',span=(446,512),font='NotoSerifKR[wght].ttf')
  put(d,T['Extras_Title'],im); prev['Extras_Title']=im
  # Title_Parts PRESS START x3
  im=decode(bytes(d),T['Title_Parts']).convert('RGBA'); src=im.copy()
  for b in [(651,8,921,42),(651,59,921,91),(650,107,922,143)]:
    replace(im,src,b,'START 버튼을 누르십시오',align='center',span=(600,1020),measure=meas(b[2]-b[0],b[3]-b[1],6,28))
  put(d,T['Title_Parts'],im); prev['Title_Parts']=im
  from diff import build as dbuild
  im=decode(bytes(d),T['Chap_Difficulty']); nd=dbuild(im); put(d,T['Chap_Difficulty'],nd); prev['Chap_Difficulty']=nd
  open('uimode_ko.bin','wb').write(d)
  out=pack(bytes(d),6); assert unpack(out)==bytes(d)
  open('LocalizedUIGameModePackage_ENG.mpkg','wb').write(out); print('packed',len(out))
  for k,v in prev.items():
    bg=Image.new('RGBA',v.size,(110,90,60,255)); bg.alpha_composite(v); bg.convert('RGB').save(f'prev_{k}.png')
