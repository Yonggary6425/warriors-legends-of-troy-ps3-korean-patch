import sys; sys.path.insert(0,'/tmp/claude-0/work')
from restyle import *
def build_bg2(im):
  im=im.convert('RGBA'); src=im.copy()
  plates=[((678,11,873,79),'아킬레우스','아카이아','left'),((678,97,919,165),'파트로클로스','아카이아','left'),
          ((678,183,797,251),'아이아스','아카이아','left'),((678,268,887,337),'오디세우스','아카이아','left'),
          ((831,355,1013,428),'헥토르','트로이아','right'),((881,441,1013,513),'파리스','트로이아','right'),
          ((842,527,1013,600),'아이네이아스','트로이아','right'),((743,613,1013,685),'펜테실레이아','트로이아','right')]
  for (x0,y0,x1,y1),nm,fac,al in plates:
    # widen box to available space so long Korean names fit
    X0,X1=(678,1016) if al=='left' else (672,1016)
    im.paste(Image.new('RGBA',(x1-x0+6,y1-y0+4),(0,0,0,0)),(x0-3,y0-2))
    nb=(X0,y0,X1,y0+42); fb=(X0,y0+40,X1,y1)
    # sample colours from the untouched source
    fillN,darkN=profile(src.crop((x0,y0,x1,y0+42))); fillF,darkF=profile(src.crop((x0,y0+42,x1,y1)))
    tmp=src.crop((678,11,873,53)); tmp2=src.crop((678,53,873,79))
    for box,txt,fp,s,crop in [(nb,nm,fillN,0.95,tmp),(fb,fac,fillF,0.9,tmp2)]:
      W=box[2]-box[0]; H=box[3]-box[1]
      can=Image.new('RGBA',(W,H),(0,0,0,0))
      # put original crop at matching side so draw() measures its ink span
      ox=0 if al=='left' else W-crop.width
      can.paste(crop,(ox,0 if box is nb else 2))
      draw(can,(0,0,W,H),txt,align=al,halo=3,fillprof=np.vstack([fp]+[fp[-1:]]*max(0,H-len(fp)))[:H],size=s,pad=3)
      im.alpha_composite(can,box[:2])
  # main menu items (gold col left, white col right)
  items=['이야기 계속하기','새 이야기','장 선택','부록','옵션']
  L=[(15,739,381,781),(16,797,308,840),(16,855,441,897),(16,913,211,956),(15,972,232,1014)]
  R=[(525,738,895,786),(525,796,822,844),(526,853,955,901),(526,913,726,960),(524,971,747,1019)]
  for col,xr in [(L,(8,512)),(R,(518,1020))]:
    for b,t in zip(col,items):
      draw(im,(xr[0],b[1]-2,xr[1],b[3]+2),t,align='left',clear=(b[0]-4,b[1]-3,b[2]+4,b[3]+3),pad=b[0]-xr[0]) if False else None
      crop=src.crop(b); W=xr[1]-xr[0]; H=b[3]-b[1]+4
      can=Image.new('RGBA',(W,H),(0,0,0,0)); can.paste(crop,(b[0]-xr[0],2))
      draw(can,(0,0,W,H),t,align='left',pad=b[0]-xr[0],halo=0)
      im.paste(Image.new('RGBA',(b[2]-b[0]+8,b[3]-b[1]+6),(0,0,0,0)),(b[0]-4,b[1]-3))
      im.alpha_composite(can,(xr[0],b[1]-2))
  return im
