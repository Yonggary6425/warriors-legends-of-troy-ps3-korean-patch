import numpy as np, io
from PIL import Image, ImageDraw, ImageFont, ImageFilter
FD='/tmp/claude-0/fonts/'
def style_from(orig):
  a=np.asarray(orig).astype(float)
  al=a[...,3]
  core=a[al>250][:, :3]
  rim=a[(al>60)&(al<200)][:, :3]
  return core, rim
def render_title(text,size,orig,cap=30,font='NotoSerifKR[wght].ttf',weight=900,seed=1,track=0.18,grow=2,ref_bbox=None):
  W,H=size
  core,rim=style_from(orig)
  bb0=ref_bbox or orig.getbbox()
  SS=4
  f=ImageFont.truetype(FD+font,cap*SS)
  if weight: f.set_variation_by_axes([weight])
  tmp=Image.new('L',(W*SS*2,H*SS*2),0); d=ImageDraw.Draw(tmp)
  xx=W*SS//2
  for ch in text:
    d.text((xx,H*SS//2),ch,font=f,fill=255)
    xx+=int(f.getlength(ch)+track*cap*SS)
  bb=tmp.getbbox(); m=tmp.crop(bb)
  # scale to target height of original ink box
  th=(bb0[3]-bb0[1])+grow
  sc=th/(m.height/SS)
  mw=int(m.width/SS*sc); mh=int(m.height/SS*sc)
  maxw=W-16
  if mw>maxw: mh=int(mh*maxw/mw); mw=maxw
  m=m.resize((mw,mh),Image.LANCZOS)
  A=Image.new('L',(W,H),0)
  x=(W-mw)//2; y=(bb0[1]+bb0[3])//2-mh//2
  A.paste(m,(x,y))
  Af=np.asarray(A).astype(float)/255
  rng=np.random.default_rng(seed)
  # fill: sample core colors randomly (grungy texture)
  idx=rng.integers(0,len(core),(H,W)); fill=core[idx]
  fill=np.asarray(Image.fromarray(fill.astype('uint8')).filter(ImageFilter.GaussianBlur(0.4))).astype(float)
  if len(rim):
    L=rim.sum(1); rimc=rim[L>=np.percentile(L,70)].mean(0)
  else: rimc=np.array([220,120,60])
  glow=np.asarray(A.filter(ImageFilter.MaxFilter(3)).filter(ImageFilter.GaussianBlur(0.6))).astype(float)/255
  shadow=np.asarray(A.filter(ImageFilter.MaxFilter(3)).filter(ImageFilter.GaussianBlur(1.0))).astype(float)/255
  shadow=np.roll(np.roll(shadow,1,0),1,1)
  out=np.zeros((H,W,4))
  ga=np.clip(glow*0.85,0,1); sa=np.clip(shadow*0.6,0,1)
  # composite: shadow (black) under glow(rim color) under fill
  col=np.zeros((H,W,3)); alpha=np.zeros((H,W))
  for c,a in [(np.zeros(3),sa),(rimc,ga),(None,Af)]:
    cc=fill if c is None else np.broadcast_to(c,(H,W,3))
    col=cc*a[...,None]+col*(1-a[...,None]); alpha=a+alpha*(1-a)
  return Image.fromarray(np.dstack([col,alpha*255]).clip(0,255).astype('uint8'),'RGBA')
