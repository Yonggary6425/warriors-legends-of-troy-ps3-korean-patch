import glob,os,json,re
SRC="/mnt/user-data/uploads/Warriors Legends of Troy [U]/PS3_GAME/USRDIR/PS3/Localization/ENG"
ents=[]
for f in sorted(glob.glob(glob.escape(SRC)+'/*.ini')):
  name=os.path.basename(f)
  raw=open(f,'rb').read()
  t=raw.decode('utf-16-be')
  sec=None
  for i,l in enumerate(t.lstrip('﻿').split('\r\n')):
    if l.startswith('[') and l.endswith(']'): sec=l[1:-1]; continue
    if not l or l.startswith(';') or '=' not in l: continue
    k,v=l.split('=',1)
    ents.append(dict(id=len(ents),file=name,sec=sec,key=k,line=i,en=v))
json.dump(ents,open('src.json','w'),ensure_ascii=False,indent=0)
print(len(ents))
# checks
print(sum(1 for e in ents if not e['en'].strip()),'empty')
uniq={}
for e in ents: uniq.setdefault(e['en'],[]).append(e['id'])
print(len(uniq),'unique texts')
# line endings / BOM check
for f in sorted(glob.glob(glob.escape(SRC)+'/*.ini'))[:3]:
  r=open(f,'rb').read(); print(os.path.basename(f), r[:2].hex(), r.count(b'\x00\r\x00\n'), r.count(b'\x00\n'))
