import numpy as np, math
from PIL import Image, ImageDraw, ImageFont, ImageFilter
FD='/tmp/claude-0/fonts/'
RINGS=[(69.4,65.6,41.3),(185.6,65.9,41.0),(300.7,65.8,40.5),(417.6,65.5,40.0)]
BANDS=[(0,125),(125,245),(245,370),(370,512)]
def build(im,labels=('쉬움','보통','어려움','달인')):
  im=im.convert('RGBA'); a=np.asarray(im).astype(float); H,W=a.shape[:2]
  yy,xx=np.mgrid[0:H,0:W]
  out=a.copy()
  al=a[...,3]; L=a[...,:3].sum(-1)
  core=a[(al>200)&(L<200)][:,:3].mean(0)          # dark brown
  glowc=a[(al>40)&(al<160)&(L>350)][:,:3].mean(0) if ((al>40)&(al<160)&(L>350)).any() else np.array([230,215,180])
  layer=np.zeros((H,W,4))
  for (cx,cy,r),(bx0,bx1),txt in zip(RINGS,BANDS,labels):
    d=np.hypot(xx-cx,yy-cy); band=(xx>=bx0)&(xx<bx1)
    txtmask=band&(d>r+5)&(al>0)
    core_t=band&(d>r+8)&(d<r+24)&(al>150)
    ys,xs=np.where(core_t)
    ang=np.degrees(np.arctan2(ys-cy,xs-cx))
    a0,a1=np.percentile(ang,5),np.percentile(ang,95)
    rad=np.median(d[core_t])
    out[txtmask]=0
    # draw korean along arc
    n=len(txt); fs=19 if n<=2 else 17
    f=ImageFont.truetype(FD+'NotoSerifKR[wght].ttf',fs*4); f.set_variation_by_axes([900])
    span=a1-a0; mid=(a0+a1)/2
    step=min(span/max(n,1), 360*(fs*1.15)/(2*math.pi*rad))
    angs=[mid+(i-(n-1)/2)*step for i in range(n)]
    can=Image.new('L',(W*4,H*4),0)
    for ch,th in zip(txt,angs):
      g=Image.new('L',(fs*8,fs*8),0); ImageDraw.Draw(g).text((fs*4,fs*4),ch,font=f,fill=255,anchor='mm')
      g=g.rotate(-(th+90),resample=Image.BICUBIC)
      px=cx+rad*math.cos(math.radians(th)); py=cy+rad*math.sin(math.radians(th))
      can.paste(g,(int(px*4-fs*4),int(py*4-fs*4)),g)
    A=can.resize((W,H),Image.LANCZOS)
    Af=np.asarray(A).astype(float)/255
    gl=np.asarray(A.filter(ImageFilter.MaxFilter(5)).filter(ImageFilter.GaussianBlur(1.5))).astype(float)/255*0.75
    for c,aa in [(glowc,gl),(core,Af)]:
      layer[...,:3]=c*aa[...,None]+layer[...,:3]*(1-aa[...,None]); layer[...,3]=aa*255+layer[...,3]*(1-aa)
  base=Image.fromarray(out.clip(0,255).astype('uint8'),'RGBA')
  base.alpha_composite(Image.fromarray(layer.clip(0,255).astype('uint8'),'RGBA'))
  return base
