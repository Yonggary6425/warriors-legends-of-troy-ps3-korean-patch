import struct,zlib,sys
def unpack(d):
  n=struct.unpack('>I',d[:4])[0]; p=4*n; out=[]
  for i in range(n):
    sz=struct.unpack('>I',d[p:p+4])[0]; p+=4
    out.append(zlib.decompress(d[p:p+sz])); p+=sz
  assert len(d)-p<16 and not d[p:].strip(b"\0"),(p,len(d))
  return b''.join(out)
if __name__=='__main__':
  open(sys.argv[2],'wb').write(unpack(open(sys.argv[1],'rb').read()))

def pack(raw,level=9):
  chunks=[zlib.compress(raw[i:i+65536],level) for i in range(0,len(raw),65536)]
  n=len(chunks); body=b''; offs=[]
  for c in chunks:
    offs.append(len(body)); body+=struct.pack('>I',len(c))+c
  return struct.pack('>I',n)+struct.pack('>%dI'%(n-1),*offs[1:])+body+b'\0\0\0\0'

def chunks_of(d):
  n=struct.unpack('>I',d[:4])[0]; p=4*n; out=[]
  for i in range(n):
    sz=struct.unpack('>I',d[p:p+4])[0]; c=d[p+4:p+4+sz]; out.append(c); p+=4+sz
  return out
def pack_reuse(raw,orig,level=9):
  oc=chunks_of(orig); reused=0
  chunks=[]
  for i in range(0,len(raw),65536):
    seg=raw[i:i+65536]; k=i//65536
    if k<len(oc) and zlib.decompress(oc[k])==seg: chunks.append(oc[k]); reused+=1
    else: chunks.append(zlib.compress(seg,level))
  n=len(chunks); body=b''; offs=[]
  for c in chunks:
    offs.append(len(body)); body+=struct.pack('>I',len(c))+c
  return struct.pack('>I',n)+struct.pack('>%dI'%(n-1),*offs[1:])+body+b'\0\0\0\0', reused, n
