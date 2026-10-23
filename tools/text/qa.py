import json,glob,re,collections
E=json.load(open('src.json')); byid={e['id']:e for e in E}
canon=json.load(open('canon.json'))
ko={}
for f in sorted(glob.glob('out/*.tsv')):
  for ln in open(f,encoding='utf-8').read().split('\n'):
    if not ln: continue
    i,t=ln.split('\t',1); i=int(i)
    if i in ko: print('DUP',i)
    ko[i]=t
TAG=re.compile(r'<[^>]+>|\{(?![’‘—…™])[^}]+\}|%\d*[ds]|%0\d+d')
miss=[];probs=[]
for en,cid in canon.items():
  if cid not in ko:
    stripped=TAG.sub('',en)
    if re.search('[A-Za-z]',stripped.replace('{’}','').replace('{…}','')): miss.append((cid,en))
    else: ko[cid]=en
    continue
  t=ko[cid]
  a=collections.Counter(TAG.findall(en)); b=collections.Counter(TAG.findall(t))
  if a!=b: probs.append(('TAG',cid,en,t))
  if re.search('[A-Za-z]{3,}',TAG.sub('',t)) and not t.startswith('Warriors'): probs.append(('LATIN',cid,t))
  if t.count('\\n')>en.count('\\n')+0: probs.append(('NL+',cid,t))
print('translated',len(ko),'missing',len(miss)); [print(m) for m in miss[:20]]
for p in probs: print(p)
# build full mapping per entry id
full={}
for e in E:
  if not e['en'].strip(): full[e['id']]=e['en']; continue
  full[e['id']]=ko.get(canon[e['en']],e['en'])
json.dump(full,open('ko_full.json','w'),ensure_ascii=False)
# line length stats for subtitles
L=[]
for e in E:
  if e['file'].startswith('SubTitle'):
    for seg in full[e['id']].split('\\n'): L.append((len(seg),e['id'],seg))
L.sort(reverse=True); print('longest subtitle lines:'); [print(x) for x in L[:8]]
