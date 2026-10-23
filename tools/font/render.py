import sys; sys.path.insert(0,'/tmp/claude-0/work')
from g1n import parse
from PIL import Image
def glyph_img(g,fi,gi,hi_first=True):
  w,h,a,b,adv,yo,c,lh,off=g['fonts'][fi]['glyphs'][gi]
  d=g['raw']; base=g['gdata']+off
  st=(w+1)//2; px=[]
  for y in range(h):
    row=[]
    for byte in d[base+y*st:base+(y+1)*st]:
      row+=[byte>>4,byte&15]
    px+=row[:w]
  im=Image.new('L',(max(w,1),h))
  if w: im.putdata([p*17 for p in px[:w*h]])
  return im,(w,h,a,b,adv,yo,c,lh)
def sheet(g,fi,chars,scale=2,hi_first=True):
  f=g['fonts'][fi]; ims=[]
  for ch in chars:
    gi=f['map'][ord(ch)] if ord(ch)<65536 else 0
    if gi==0 and ord(ch)!=0: ims.append(None); continue
    ims.append(glyph_img(g,fi,gi,hi_first))
  H=max(i[0].height for i in ims if i) ; W=sum((i[1][4] if i else 8)+1 for i in ims)
  out=Image.new('L',(W+4,H+4)); x=2
  for i in ims:
    if i: out.paste(i[0],(x+i[1][2],2)); x+=i[1][4]+1
    else: x+=9
  return out.resize((out.width*scale,out.height*scale),Image.NEAREST)
