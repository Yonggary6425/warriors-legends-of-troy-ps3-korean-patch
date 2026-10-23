import sys,io; sys.path.insert(0,'/tmp/claude-0/work')
from restyle import *; from g1t import *; from cpkg import pack, unpack
from build_ui import put
d=bytearray(open('localized_ko.bin','rb').read()); T={t['name']:t for t in scan(bytes(d))}
prev=[]
def do(name,items,span):
  im=decode(bytes(d),T[name]).convert('RGBA'); src=im.copy()
  for b,t in items: replace(im,src,b,t,align='center',span=span(b),size=0.8 if t=='목표' else 1.0)
  put(d,T[name],im); prev.append(im)
do('Pause_Title',[((55,3,158,37),'격자'),((248,4,479,37),'구매'),((136,53,377,95),'목표'),((182,108,331,141),'기술'),((165,155,344,188),'옵션'),((174,203,335,235),'시스템')],
   lambda b:(b[0]-30,b[2]+30) if b[1]<40 else (60,452))
do('Result_Text',[((59,7,453,40),'장 결과'),((89,58,423,91),'종합 결과')],lambda b:(30,482))
do('Game_Over_Text',[((125,6,390,39),'패배')],lambda b:(60,452))
do('ChallengeResult_Text',[((49,18,462,51),'도전 결과')],lambda b:(30,482))
do('Options_Title',[((44,18,210,51),'옵션')],lambda b:(20,236))
open('localized_ko2.bin','wb').write(d)
out=pack(bytes(d)); assert unpack(out)==bytes(d); open('LocalizedCommonPackage_ENG.cpkg','wb').write(out); print('loc packed',len(out))
# common: KLEOS (rotated) in Fury_Parts
c=bytearray(open('common.bin','rb').read()); TC={t['name']:t for t in scan(bytes(c))}
im=decode(bytes(c),TC['Fury_Parts']).convert('RGBA')
reg=(478,26,508,126)
piece=im.crop(reg).rotate(90,expand=True)   # vertical -> horizontal
src=piece.copy()
bx=boxes(piece,jx=4,jy=2); print('kleos box',bx)
b=max(bx,key=lambda b:(b[2]-b[0])*(b[3]-b[1]))
replace(piece,src,b,'클레오스',align='left',span=(b[0],piece.width-2))
im.paste(Image.new('RGBA',(reg[2]-reg[0],reg[3]-reg[1]),(0,0,0,0)),reg[:2]); im.alpha_composite(piece.rotate(-90,expand=True),reg[:2])
put(c,TC['Fury_Parts'],im); prev.append(im)
out=pack(bytes(c),6); assert unpack(out)==bytes(c); open('CommonMasterPackage.cpkg','wb').write(out); print('common packed',len(out))
for i,p in enumerate(prev):
  bg=Image.new('RGBA',p.size,(110,90,60,255)); bg.alpha_composite(p); bg.convert('RGB').save(f'prevloc_{i}.png')
