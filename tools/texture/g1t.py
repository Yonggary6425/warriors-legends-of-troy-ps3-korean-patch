import struct,io,re
from PIL import Image
FMT={0x06:'DXT1',0x08:'DXT5',0x07:'DXT3'}
def dds(w,h,fourcc,data):
  hdr=struct.pack('<4s7I44s2I4s5I5I', b'DDS ',124,0x81007,h,w,len(data),0,1,b'\0'*44,32,4,fourcc.encode(),0,0,0,0,0,0x1000,0,0,0,0)
  return hdr+data
def parse(d,p):
  magic,ver,size,toff,n=struct.unpack('>4s4sIII',d[p:p+20])
  tex=p+toff+struct.unpack('>I',d[p+toff:p+toff+4])[0]
  b=d[tex:tex+16]; fmt=b[1]; dim=b[2]; w=1<<(dim>>4); h=1<<(dim&15)
  ext=struct.unpack('>I',d[tex+8:tex+12])[0] if b[7] else 0
  hl=8+ext
  bpp={0x06:8,0x08:16,0x07:16}[fmt]
  n=(w//4)*(h//4)*bpp
  return dict(p=p,size=size,tex=tex,fmt=fmt,w=w,h=h,dataoff=tex+hl,datalen=n)
def decode(d,t):
  raw=d[t['dataoff']:t['dataoff']+t['datalen']]
  return Image.open(io.BytesIO(dds(t['w'],t['h'],FMT[t['fmt']],raw))).convert('RGBA')
def scan(d):
  out=[]
  for m in re.finditer(b'G1TG',d):
    p=m.start(); nm=re.findall(rb'(?:\x00[\x20-\x7e]){4,}',d[max(0,p-80):p])
    nm=nm[-1].decode('utf-16-be') if nm else hex(p)
    try: t=parse(d,p); t['name']=nm; out.append(t)
    except Exception as e: print('fail',nm,e)
  return out
