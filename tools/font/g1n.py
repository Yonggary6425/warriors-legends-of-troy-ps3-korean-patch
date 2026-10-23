import struct,sys
def parse(d):
  magic,fsz,tbl0,one,gdata,npal,nf=struct.unpack('>8s6I',d[:0x20])
  offs=list(struct.unpack('>%dI'%nf,d[0x20:0x20+4*nf]))
  pals=[]
  for p in range(npal):
    o=0x3c+p*64
    pals.append([tuple(d[o+i*4:o+i*4+4]) for i in range(16)])
  fonts=[]
  ends=offs[1:]+[gdata]
  for o,e in zip(offs,ends):
    m=struct.unpack('>65536H',d[o:o+0x20000])
    ng=(e-o-0x20000)//12
    ents=[struct.unpack('>8BI',d[o+0x20000+i*12:o+0x20000+i*12+12]) for i in range(ng)]
    fonts.append(dict(off=o,map=m,glyphs=ents))
  return dict(fsz=fsz,gdata=gdata,npal=npal,pals=pals,fonts=fonts,raw=d)
if __name__=='__main__':
  d=open(sys.argv[1],'rb').read(); g=parse(d)
  print('gdata',hex(g['gdata']),'npal',g['npal'])
  for i,f in enumerate(g['fonts']):
    nz=[(c,v) for c,v in enumerate(f['map']) if v]
    rng=(hex(nz[0][0]),hex(nz[-1][0])) if nz else None
    ws=set((e[0],e[1]) for e in f['glyphs'])
    print(i,'glyphs',len(f['glyphs']),'mapped',len(nz),'range',rng,'sizes',sorted(ws)[:6], 'first',f['glyphs'][1] if len(f['glyphs'])>1 else None)
