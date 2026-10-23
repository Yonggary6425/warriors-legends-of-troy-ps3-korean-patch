import json,glob,os
SRC="/mnt/user-data/uploads/Warriors Legends of Troy [U]/PS3_GAME/USRDIR/PS3/Localization/ENG"
OUT="/tmp/claude-0/tr/build/PS3_GAME/USRDIR/PS3/Localization/ENG"
os.makedirs(OUT,exist_ok=True)
E=json.load(open('src.json')); full={int(k):v for k,v in json.load(open('ko_full.json')).items()}
byfile={}
for e in E: byfile.setdefault(e['file'],{})[e['line']]=(e['key'],full[e['id']])
for f in sorted(glob.glob(glob.escape(SRC)+'/*.ini')):
  n=os.path.basename(f); t=open(f,'rb').read().decode('utf-16-be')
  bom=t.startswith('﻿'); body=t.lstrip('﻿')
  lines=body.split('\r\n')
  for i,(k,v) in byfile.get(n,{}).items():
    assert lines[i].split('=',1)[0]==k,(n,i)
    lines[i]=k+'='+v
  out=('﻿' if bom else '')+'\r\n'.join(lines)
  open(os.path.join(OUT,n),'wb').write(out.encode('utf-16-be'))
print(len(os.listdir(OUT)))
