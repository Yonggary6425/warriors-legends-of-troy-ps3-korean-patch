import sys,struct,json
sys.path.insert(0,'/tmp/claude-0/work')
import numpy as np
from g1n import parse
import hangul
SRC='/mnt/user-data/uploads/Warriors Legends of Troy [U]/PS3_GAME/USRDIR/PS3/TrojanMusou/Fonts/fonts.g1n'
d=open(SRC,'rb').read(); g=parse(d)
chars=open('/tmp/claude-0/tr/charset.txt',encoding='utf-8').read().strip()+'「」'
SPACE={1:9,3:8}
nf=len(g['fonts'])
fonts=[]
gdata=bytearray(d[g['gdata']:])
for fi,f in enumerate(g['fonts']):
  m=list(f['map']); ents=[list(e) for e in f['glyphs']]
  if fi in SPACE:
    gi=m[32]; ents[gi][0]=SPACE[fi]; ents[gi][4]=SPACE[fi]
  if fi<4:
    tmpl=ents[m[ord('A')]]
    for ch in chars:
      idx=hangul.render(ch,fi)
      h,w=idx.shape
      st=(w+1)//2; buf=bytearray()
      for y in range(h):
        row=list(idx[y])+[0]*(st*2-w)
        buf+=bytes((row[i]<<4)|row[i+1] for i in range(0,st*2,2))
      off=len(gdata); gdata+=buf
      e=[w,h,0,h,max(1,w-(4 if fi==3 else 2)),tmpl[5],tmpl[6],tmpl[7],off]
      m[ord(ch)]=len(ents); ents.append(e)
  fonts.append((m,ents))
# assemble
hdr_end=0x3c+g['npal']*64
out=bytearray(d[:hdr_end])
offs=[]
body=bytearray()
for m,ents in fonts:
  offs.append(hdr_end+len(body))
  body+=struct.pack('>65536H',*m)
  for e in ents: body+=struct.pack('>8BI',*[v&0xff for v in e[:8]],e[8])
gd=hdr_end+len(body)
out+=body+gdata
struct.pack_into('>I',out,0x08,len(out))
struct.pack_into('>I',out,0x0c,offs[0])
struct.pack_into('>I',out,0x14,gd)
struct.pack_into('>%dI'%nf,out,0x20,*offs)
open('fonts_ko.g1n','wb').write(out)
print('size',len(out),'orig',len(d),'glyphs',[len(e) for _,e in fonts])
