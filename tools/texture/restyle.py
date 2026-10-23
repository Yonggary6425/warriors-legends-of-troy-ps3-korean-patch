import numpy as np
from PIL import Image, ImageDraw, ImageFont, ImageFilter
from scipy import ndimage
FD='/tmp/claude-0/fonts/'
def boxes(im,region=None,thr=40,jx=10,jy=2,minarea=60):
  a=np.asarray(im)[...,3]
  x0,y0,x1,y1=region or (0,0,im.width,im.height)
  m=a[y0:y1,x0:x1]>thr
  m2=ndimage.binary_dilation(m,structure=np.ones((2*jy+1,2*jx+1)))
  lab,n=ndimage.label(m2); out=[]
  for s in ndimage.find_objects(lab):
    sub=m[s]
    if sub.sum()<minarea: continue
    ys,xs=np.where(sub)
    out.append((int(x0+s[1].start+xs.min()),int(y0+s[0].start+ys.min()),int(x0+s[1].start+xs.max()+1),int(y0+s[0].start+ys.max()+1)))
  return sorted(out,key=lambda b:(b[1],b[0]))
def split_lines(im,box,thr=60):
  a=np.asarray(im.crop(box))[...,3]
  rows=(a>thr).sum(1)>0
  lines=[];start=None
  for i,r in enumerate(rows):
    if r and start is None: start=i
    if not r and start is not None: lines.append((start,i)); start=None
  if start is not None: lines.append((start,len(rows)))
  # merge tiny gaps
  out=[]
  for l in lines:
    if out and l[0]-out[-1][1]<=2: out[-1]=(out[-1][0],l[1])
    else: out.append(l)
  return [(box[0],box[1]+a0,box[2],box[1]+a1) for a0,a1 in out]
def profile(crop):
  a=np.asarray(crop).astype(float); al=a[...,3]; rgb=a[...,:3]; L=rgb.sum(-1)
  H=a.shape[0]; fill=np.zeros((H,3)); ok=np.zeros(H,bool)
  for y in range(H):
    sel=(al[y]>230)
    if sel.sum()>3:
      v=rgb[y][sel]; l=L[y][sel]; top=v[l>=np.percentile(l,60)]
      fill[y]=top.mean(0); ok[y]=True
  if ok.any():
    idx=np.arange(H)
    for c in range(3): fill[:,c]=np.interp(idx,idx[ok],fill[ok,c])
  sel=al>230
  dark=rgb[sel][L[sel]<=np.percentile(L[sel],15)].mean(0) if sel.sum()>10 else np.array([20,10,5])
  return fill,dark
def draw(atlas,box,text,align='left',font='NotoSerifKR[wght].ttf',weight=900,track=0.06,glow=None,pad=1,clear=None,halo=0,size=1.0,fillprof=None,darkc=None):
  """replace sprite in box with Korean text in the original's colours"""
  crop=atlas.crop(box); W=box[2]-box[0]; H=box[3]-box[1]
  fill,dark=profile(crop)
  if fillprof is not None: fill=fillprof
  if darkc is not None: dark=darkc
  # ink vertical span of original (caps) to set size
  ac=np.asarray(crop).astype(float); Lc=ac[...,:3].sum(-1)
  bright=(ac[...,3]>200)&(Lc>0.45*Lc[ac[...,3]>200].max() if (ac[...,3]>200).any() else Lc>0)
  rows=np.where(bright.sum(1)>1)[0]
  ty0,ty1=(rows.min(),rows.max()+1) if len(rows) else (0,H)
  if ty1-ty0<0.4*H:
    r2=np.where((ac[...,3]>120).sum(1)>1)[0]
    ty0,ty1=(r2.min()+2,r2.max()-1) if len(r2) else (0,H)
  th=ty1-ty0
  if size!=1.0:
    c=(ty0+ty1)/2; th=int(th*size); ty0=int(c-th/2); ty1=ty0+th
  SS=4; f=ImageFont.truetype(FD+font,int(th*1.25*SS))
  if weight: f.set_variation_by_axes([weight])
  tmp=Image.new('L',(int(len(text)*th*2*SS+200),th*4*SS),0); dd=ImageDraw.Draw(tmp); xx=50
  for ch in text:
    dd.text((xx,th*SS),ch,font=f,fill=255); xx+=int(f.getlength(ch)+track*th*1.25*SS)
  bb=tmp.getbbox(); m=tmp.crop(bb)
  mh=th; mw=int(m.width*mh/m.height)
  maxw=W-2*pad-2
  if mw>maxw: mw=maxw
  m=m.resize((max(1,mw),mh),Image.LANCZOS)
  A=Image.new('L',(W,H),0)
  x={'left':pad,'right':W-pad-1-mw,'center':(W-mw)//2}[align]
  A.paste(m,(x,ty0))
  Af=np.asarray(A).astype(float)/255
  ring=np.asarray(A.filter(ImageFilter.MaxFilter(3))).astype(float)/255
  sh=np.asarray(A.filter(ImageFilter.MaxFilter(3)).filter(ImageFilter.GaussianBlur(1.0))).astype(float)/255
  sh=np.roll(np.roll(sh,1,0),1,1)*0.7
  col=np.zeros((H,W,3)); al=np.zeros((H,W))
  fl=np.broadcast_to(fill[:,None,:],(H,W,3))
  layers=[(np.zeros(3),sh),(dark,ring)]
  if halo:
    hl=np.asarray(A.filter(ImageFilter.MaxFilter(2*halo+1)).filter(ImageFilter.GaussianBlur(halo*0.6))).astype(float)/255
    layers=[(np.zeros(3),np.clip(hl*1.1,0,0.95)),(dark,ring)]
  if glow is not None: layers.insert(1,(np.array(glow,float),np.asarray(A.filter(ImageFilter.MaxFilter(5)).filter(ImageFilter.GaussianBlur(0.8))).astype(float)/255*0.8))
  for c,aa in layers+[(None,Af)]:
    cc=fl if c is None else np.broadcast_to(c,(H,W,3))
    col=cc*aa[...,None]+col*(1-aa[...,None]); al=aa+al*(1-aa)
  new=Image.fromarray(np.dstack([col,al*255]).clip(0,255).astype('uint8'),'RGBA')
  # clear original area then paste
  cb=clear or box
  atlas.paste(Image.new('RGBA',(cb[2]-cb[0],cb[3]-cb[1]),(0,0,0,0)),cb[:2])
  atlas.paste(new,box[:2])
  return atlas

def replace(im,src,box,text,align='left',span=None,halo=0,size=1.0,measure=None,font='NotoSerifKR[wght].ttf',weight=900,track=0.06):
  """clear original sprite at box and draw Korean in its style; span=(X0,X1) horizontal room"""
  x0,y0,x1,y1=box; X0,X1=span or (x0,x1)
  crop=src.crop(box); W=X1-X0; H=y1-y0+4
  ca=np.asarray(crop).astype(float); amax=ca[...,3].max()
  if amax<235:
    ca[...,3]=np.clip(ca[...,3]*255/max(amax,1),0,255); crop=Image.fromarray(ca.astype('uint8'),'RGBA')
  can=Image.new('RGBA',(W,H),(0,0,0,0))
  ref=measure if measure is not None else crop
  if align=='left': ox=x0-X0
  elif align=='right': ox=W-(X1-x1)-ref.width
  else: ox=(W-ref.width)//2
  can.paste(ref,(ox,2))
  fp,dk=profile(crop)
  if fp.shape[0]!=ref.height: fp=None
  pad={'left':x0-X0,'right':X1-x1,'center':0}[align]
  draw(can,(0,0,W,H),text,align=align,pad=max(pad,1),halo=halo,size=size,fillprof=None if fp is None else np.vstack([fp[:1]]*2+[fp]+[fp[-1:]]*2)[:H],font=font,weight=weight,track=track,darkc=dk)
  if amax<235:
    cc=np.asarray(can).astype(float); cc[...,3]*=amax/255; can=Image.fromarray(cc.astype('uint8'),'RGBA')
  im.paste(Image.new('RGBA',(x1-x0+12,y1-y0+10),(0,0,0,0)),(x0-6,y0-5))
  im.alpha_composite(can,(X0,y0-2))
  return im
