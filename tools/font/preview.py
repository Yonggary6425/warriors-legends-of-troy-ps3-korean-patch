import sys; sys.path.insert(0,'/tmp/claude-0/work')
import numpy as np
from PIL import Image
from g1n import parse
from hangul import render
F='/mnt/user-data/uploads/Warriors Legends of Troy [U]/PS3_GAME/USRDIR/PS3/TrojanMusou/Fonts/fonts.g1n'
G=parse(open(F,'rb').read())
def orig_idx(g,fi,ch):
  f=g['fonts'][fi]; gi=f['map'][ord(ch)]
  if not gi: return None
  w,h,a,b,adv,yo,c,lh,off=f['glyphs'][gi]; d=g['raw']; base=g['gdata']+off; st=(w+1)//2
  arr=np.zeros((h,max(w,1)),int)
  for y in range(h):
    r=[]
    for by in d[base+y*st:base+(y+1)*st]: r+=[by>>4,by&15]
    arr[y,:w]=r[:w]
  return arr
def line(g,fi,text,pal,newfn=None,bg=(40,38,35)):
  parts=[]
  for ch in text:
    a=orig_idx(g,fi,ch)
    if a is None and newfn: a=newfn(ch,fi)
    if a is None: a=np.zeros((g['fonts'][fi]['glyphs'][1][1],6),int)
    parts.append(a)
  H=max(p.shape[0] for p in parts); W=sum(p.shape[1] for p in parts)
  idx=np.zeros((H,W),int); x=0
  for p in parts: idx[:p.shape[0],x:x+p.shape[1]]=p; x+=p.shape[1]
  P=np.array([[c[1],c[2],c[3],c[0]] for c in g['pals'][pal]],float)  # ARGB->RGBA
  rgba=P[idx]; a=rgba[...,3:4]/255
  out=rgba[...,:3]*a+np.array(bg)*(1-a)
  return Image.fromarray(out.astype('uint8'))
