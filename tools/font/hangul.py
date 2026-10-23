import numpy as np
from PIL import Image, ImageDraw, ImageFont, ImageFilter
FD='/tmp/claude-0/fonts/'
SS=4
def mask(ch,fontfile,px,weight=None):
  f=ImageFont.truetype(FD+fontfile,px*SS)
  if weight: f.set_variation_by_axes([weight])
  W=px*SS*2
  im=Image.new('L',(W,W),0); ImageDraw.Draw(im).text((px*SS//2,px*SS//2),ch,font=f,fill=255)
  return im
def ref_box(fontfile,px,weight=None):
  # vertical extent of a tall reference set, so all syllables share one baseline
  ys=[]
  for ch in '한글각뷁흙':
    b=mask(ch,fontfile,px,weight).getbbox(); ys+= [b[1],b[3]]
  return min(ys),max(ys)
def dilate(a,r):
  out=a.copy()
  for dy in range(-r,r+1):
    for dx in range(-r,r+1):
      if dx*dx+dy*dy<=r*r+r: out=np.maximum(out,np.roll(np.roll(a,dy,0),dx,1))
  return out
def shift(a,dx,dy): return np.roll(np.roll(a,dy,0),dx,1)
def blur(a,r):
  im=Image.fromarray((np.clip(a,0,1)*255).astype('uint8')).filter(ImageFilter.GaussianBlur(r))
  return np.asarray(im)/255.
class Style:
  def __init__(s,**k): s.__dict__.update(k)
STYLES={
 0:Style(font='IBMPlexSansKR-SemiBold.ttf',weight=None,cell=22,top=2,bot=17,pad=1,outline=1,shadow=(1,2),shblur=0.6,shalpha=0.8,kind='outline',gap=1),
 1:Style(font='GowunDodum-Regular.ttf',weight=None,cell=30,top=4,bot=25,bold=1,pad=1,outline=1,shadow=(1,1),shblur=0.8,shalpha=0.55,kind='soft',gap=1),
 2:Style(font='GowunDodum-Regular.ttf',weight=None,cell=22,top=2,bot=17,bold=0,pad=1,outline=1,shadow=(1,1),shblur=0.6,shalpha=0.45,kind='soft',gap=1),
 3:Style(font='NotoSerifKR[wght].ttf',weight=900,cell=30,top=7,bot=27,bold=0,pad=2,outline=1,shadow=(1,2),shblur=0.8,shalpha=1.0,kind='bevel',gap=1),
}
_cache={}
def render(ch,fi):
  st=STYLES[fi]; H=st.cell
  key=fi
  if key not in _cache:
    # find pixel size so reference glyph height maps to top..bot
    target=(st.bot-st.top+1)
    px=H
    y0,y1=ref_box(st.font,px,st.weight)
    scale=target*SS/(y1-y0); px2=max(6,round(px*scale))
    y0,y1=ref_box(st.font,px2,st.weight)
    _cache[key]=(px2,y0,y1)
  px,y0,y1=_cache[key]
  m=mask(ch,st.font,px,st.weight)
  if getattr(st,'bold',0): m=m.filter(ImageFilter.MaxFilter(2*st.bold+1))
  bb=m.getbbox()
  if bb is None: return None
  x0,x1=bb[0],bb[2]
  # crop horizontally to ink, vertically to reference box mapped to rows top..bot
  pad=(st.pad+2)*SS
  crop=m.crop((x0-pad, y0-st.top*SS, x1+pad, y0-st.top*SS+H*SS))
  A=np.asarray(crop.resize((crop.width//SS,H),Image.BOX)).astype(float)/255
  # trim columns: keep 1+pad columns around ink
  cols=np.where(A.max(0)>0.12)[0]
  l=max(0,cols[0]-st.pad); r=min(A.shape[1],cols[-1]+st.pad+1+max(st.shadow[0],0))
  A=A[:,l:r]
  O=np.clip(dilate(A,st.outline),0,1)
  S=blur(shift(np.pad(O,((0,0),(0,0))),*st.shadow),st.shblur)*st.shalpha
  idx=np.zeros(A.shape,int)
  if st.kind in('outline','soft'):
    oa = 1.0 if st.kind=='outline' else 0.55
    out=np.maximum(O*oa,S)
    idx=np.rint(out*7).astype(int)
    fill=A>0.08
    idx[fill]=7+np.clip(np.rint(A[fill]*7),1,7).astype(int)
  else: # bevel
    halo=np.maximum(dilate(O,1)*0.5,S)
    idx=np.where(halo>0.05,3,0)
    idx=np.where((halo>0.05)&(halo<0.25),1,idx)
    # bevel lighting from blurred alpha gradient
    Bm=blur(A,1.0); gy,gx=np.gradient(Bm)
    light=-(gx*-0.7+gy*-0.7)  # light from top-left
    lum=np.clip(0.8+light*3.0,0,1)
    ring=(O>0.3)&(A<0.5)
    idx[ring]=8
    fill=A>=0.5
    idx[fill]=np.clip(np.rint(9+lum[fill]*5),9,14).astype(int)
  return idx
def to_img(idx,scale=3):
  return Image.fromarray((idx*17).astype('uint8')).resize((idx.shape[1]*scale,idx.shape[0]*scale),Image.NEAREST)
