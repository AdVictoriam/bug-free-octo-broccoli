#!/usr/bin/env python3
"""Read canonical Script JSON on stdin; write a two-PDF ZIP on stdout.
No network, custom fonts, image decoding, shell interpolation or user HTML.
ReportLab is used only for exports; the API and agent runtime are Go.
"""
from __future__ import annotations
import io
import json
import math
import sys
import zipfile
from html import escape
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
    KeepTogether, Flowable,
)

BLUE = colors.HexColor('#2465A7')
RED = colors.HexColor('#B43B40')
PURPLE = colors.HexColor('#8242BB')
INK = colors.HexColor('#20242B')
MUTED = colors.HexColor('#6B7280')
RULE = colors.HexColor('#DCE1E5')
STYLES = {
    'title': ParagraphStyle('title', fontName='Helvetica-Bold', fontSize=23, leading=28, textColor=INK, spaceAfter=14),
    'h2': ParagraphStyle('h2', fontName='Helvetica-Bold', fontSize=13, leading=17, textColor=INK, spaceBefore=12, spaceAfter=8, keepWithNext=True),
    'body': ParagraphStyle('body', fontName='Helvetica', fontSize=10.5, leading=15, textColor=INK, spaceAfter=6),
    'speech': ParagraphStyle('speech', fontName='Helvetica', fontSize=11.5, leading=16.5, textColor=INK),
    'small': ParagraphStyle('small', fontName='Helvetica', fontSize=8.5, leading=12, textColor=MUTED),
    'label': ParagraphStyle('label', fontName='Helvetica-Bold', fontSize=9, leading=13, textColor=INK),
    'delivery': ParagraphStyle('delivery', fontName='Helvetica-Oblique', fontSize=9.5, leading=13, textColor=PURPLE, spaceAfter=3),
    'blue': ParagraphStyle('blue', fontName='Helvetica', fontSize=10, leading=14, textColor=BLUE, spaceAfter=5),
    'red': ParagraphStyle('red', fontName='Helvetica', fontSize=10, leading=14, textColor=RED, spaceAfter=5),
}

def text(v: object) -> str:
    # Core PDF fonts cover Western scripts. Unsupported symbols are deliberately
    # spelled out rather than silently rendered as missing-glyph black squares.
    s = str(v).replace('★', '[second take]').replace('\x00', '')
    s = s.replace('–', '-').replace('—', ' - ').replace('…', '...')
    s = s.replace('’', "'").replace('‘', "'").replace('“', '"').replace('”', '"')
    return escape(s.encode('cp1252', errors='xmlcharrefreplace').decode('cp1252')).replace('\n', '<br/>')

def para(v: object, style: str = 'body') -> Paragraph:
    return Paragraph(text(v), STYLES[style])

class LineTag(Flowable):
    def __init__(self, number: int, star: bool):
        super().__init__()
        self.number, self.star = number, star
        self.width, self.height = 32, 29 if star else 13
    def draw(self):
        c = self.canv
        c.setFont('Helvetica-Bold', 9)
        c.setFillColor(INK)
        c.drawString(0, self.height - 10, f'L{self.number:02d}')
        if self.star:
            p = c.beginPath()
            cx, cy = 5, 6
            for i in range(10):
                radius = 5 if i % 2 == 0 else 2.15
                angle = math.pi / 2 + i * math.pi / 5
                x, y = cx + radius * math.cos(angle), cy + radius * math.sin(angle)
                if i == 0: p.moveTo(x, y)
                else: p.lineTo(x, y)
            p.close()
            c.setFillColor(PURPLE)
            c.drawPath(p, fill=1, stroke=0)

def footer(kind: str):
    def draw(c, doc):
        c.saveState()
        c.setStrokeColor(RULE)
        c.line(19*mm, 17*mm, 191*mm, 17*mm)
        c.setFont('Helvetica-Bold', 8)
        c.setFillColor(MUTED)
        c.drawString(19*mm, 12*mm, f'BOLTY  /  {kind.upper()}')
        c.setFont('Helvetica', 8)
        c.drawRightString(191*mm, 12*mm, str(doc.page))
        c.restoreState()
    return draw

def document(title: str, kind: str, content: list) -> bytes:
    buffer = io.BytesIO()
    doc = SimpleDocTemplate(buffer, pagesize=(210*mm, 297*mm),
        leftMargin=19*mm, rightMargin=19*mm, topMargin=18*mm, bottomMargin=24*mm,
        title=title, author='Bolty Studio', pageCompression=1)
    doc.build(content, onFirstPage=footer(kind), onLaterPages=footer(kind))
    return buffer.getvalue()

def actor(script: dict) -> bytes:
    out = [para('VOICE ACTOR SCRIPT', 'small'), Spacer(1, 6), para(script['title'], 'title'), para('Reading voices', 'h2')]
    rows = [[para('VOICE', 'label'), para('ROLE / PERFORMANCE', 'label')]]
    for voice in script['voices']:
        rows.append([para(voice['name']), para(f"{voice['role']} - {voice['delivery']}", 'delivery')])
    table = Table(rows, colWidths=[40*mm, 132*mm], repeatRows=1, hAlign='LEFT')
    table.setStyle(TableStyle([('VALIGN', (0,0),(-1,-1),'TOP'), ('LINEBELOW',(0,0),(-1,0),0.7,RULE), ('BOTTOMPADDING',(0,0),(-1,-1),8), ('LEFTPADDING',(0,0),(-1,-1),0)]))
    out.extend([table, Spacer(1,12)])
    for line in script['lines']:
        body = [para(line['speaker'], 'small'), para(line['delivery'], 'delivery'), para(line['text'], 'speech')]
        row = Table([[LineTag(int(line['id']), bool(line['second_take'])), body]], colWidths=[14*mm, 158*mm])
        row.setStyle(TableStyle([('VALIGN',(0,0),(-1,-1),'TOP'),('LEFTPADDING',(0,0),(-1,-1),0),('RIGHTPADDING',(0,0),(-1,-1),0),('TOPPADDING',(0,0),(-1,-1),2),('BOTTOMPADDING',(0,0),(-1,-1),10)]))
        out.append(KeepTogether([row]))
    return document(script['title'], 'Voice actor', out)

def editor(script: dict) -> bytes:
    out = [para('EDITOR SCRIPT', 'small'), Spacer(1,6), para(script['title'], 'title'), para('Thumbnail', 'h2')]
    thumb = script['thumbnail']
    for label, key in [('Left','left'), ('Right','right'), ('Central evidence','evidence'), ('Text','text')]:
        out.append(para(f'{label}: {thumb[key]}', 'blue'))
    out += [para('Tone', 'h2'), para(script['tone'], 'blue')]
    if script.get('disclosure'): out.append(para('Disclosure: ' + script['disclosure'], 'red'))
    out.append(para('Privacy and recreation', 'h2'))
    for rule in script['privacy']: out.append(para('- ' + rule, 'red'))
    out.append(para('Asset list', 'h2'))
    for asset in script['assets']:
        out.append(KeepTogether([para(asset['name'], 'label'), para(f"{asset['description']} / {asset['provenance']}", 'blue')]))
    out.append(para('Running gags', 'h2'))
    for gag in script['running_gags']: out.append(para('- ' + gag, 'blue'))
    out.append(para('Line-synced edit', 'h2'))
    for cue in script['cues']:
        block = [para(f"L{int(cue['line_id']):02d}", 'h2')]
        for label, key, style in [('Gameplay / general','gameplay','blue'), ('Edit','edit','red'), ('Sound','sound','red')]:
            if cue[key]: block.append(para(f'{label}: {cue[key]}', style))
        if cue['hold_seconds']:
            block.append(para(f"Additional non-spoken hold: {float(cue['hold_seconds']):g} seconds.", 'red'))
        out.append(KeepTogether(block))
    return document(script['title'], 'Editor', out)

def main() -> None:
    raw = sys.stdin.buffer.read(2_000_001)
    if len(raw) > 2_000_000: raise ValueError('canonical script too large')
    script = json.loads(raw)
    if not 1 <= len(script['lines']) <= 200: raise ValueError('line count outside export bounds')
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, 'w', compression=zipfile.ZIP_DEFLATED) as z:
        z.writestr('voice-actor.pdf', actor(script))
        z.writestr('editor.pdf', editor(script))
    sys.stdout.buffer.write(buffer.getvalue())

if __name__ == '__main__':
    main()
