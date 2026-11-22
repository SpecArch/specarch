package docx

import "fmt"

// stylesXML is word/styles.xml, with %s for the language. The headings are
// Word's own heading 1 to heading 6, each with its outline level, so the
// navigation pane, the table of contents and a reader's own heading
// numbering find them. The fonts are named, not embedded: Calibri, which
// a reader without it replaces with a sans serif of the same widths, and
// Courier New for code, which Word and LibreOffice have everywhere. Every element of a style is in the order the schema gives.
var stylesXML = xmlHead + `<w:styles xmlns:w="` + nsW + `">` +
	`<w:docDefaults>` +
	`<w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="21"/><w:szCs w:val="21"/><w:lang w:val="%s"/></w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="264" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
	`</w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
	`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/><w:uiPriority w:val="1"/><w:semiHidden/><w:unhideWhenUsed/></w:style>` +
	`<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:uiPriority w:val="99"/><w:semiHidden/><w:unhideWhenUsed/><w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>` +
	`<w:style w:type="numbering" w:default="1" w:styleId="NoList"><w:name w:val="No List"/><w:uiPriority w:val="99"/><w:semiHidden/><w:unhideWhenUsed/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:after="360"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="44"/><w:szCs w:val="44"/></w:rPr></w:style>` +
	heading(1, 32, 360) + heading(2, 28, 280) + heading(3, 24, 240) + heading(4, 22, 200) + heading(5, 21, 160) + heading(6, 21, 160) +
	`<w:style w:type="paragraph" w:styleId="TOCHeading"><w:name w:val="TOC Heading"/><w:basedOn w:val="Heading1"/><w:next w:val="Normal"/><w:uiPriority w:val="39"/><w:unhideWhenUsed/><w:qFormat/><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>` +
	toc(1) + toc(2) + toc(3) +
	`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:basedOn w:val="DefaultParagraphFont"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:rPr><w:color w:val="0D4099"/><w:u w:val="single"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Code Char"/><w:basedOn w:val="DefaultParagraphFont"/><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/><w:sz w:val="19"/><w:szCs w:val="19"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="CodeBlock"><w:name w:val="Code Block"/><w:basedOn w:val="Normal"/><w:pPr><w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/><w:spacing w:after="0" w:line="240" w:lineRule="auto"/><w:contextualSpacing/></w:pPr><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/><w:sz w:val="16"/><w:szCs w:val="16"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:pBdr><w:left w:val="single" w:sz="12" w:space="6" w:color="BFBFBF"/></w:pBdr><w:ind w:left="284"/></w:pPr><w:rPr><w:i/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Mark"><w:name w:val="Problem Mark"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:pPr><w:keepLines/><w:pBdr><w:left w:val="single" w:sz="24" w:space="6" w:color="CC6600"/></w:pBdr><w:shd w:val="clear" w:color="auto" w:fill="FFF2DB"/><w:ind w:left="227" w:right="57"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Rule"><w:name w:val="Rule"/><w:basedOn w:val="Normal"/><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="B3B3B3"/></w:pBdr></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Caption"><w:name w:val="caption"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="35"/><w:unhideWhenUsed/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:after="60"/></w:pPr><w:rPr><w:i/><w:color w:val="666666"/><w:sz w:val="17"/><w:szCs w:val="17"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="34"/><w:qFormat/><w:pPr><w:spacing w:after="60"/><w:ind w:left="720"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="TableText"><w:name w:val="Table Text"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="20" w:after="20" w:line="240" w:lineRule="auto"/></w:pPr><w:rPr><w:sz w:val="17"/><w:szCs w:val="17"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ProblemEntry"><w:name w:val="Problem Entry"/><w:basedOn w:val="Normal"/><w:pPr><w:tabs><w:tab w:val="left" w:pos="510"/></w:tabs><w:spacing w:after="60"/><w:ind w:left="510" w:hanging="510"/></w:pPr><w:rPr><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Header"><w:name w:val="header"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="4" w:space="4" w:color="B3B3B3"/></w:pBdr><w:spacing w:after="0"/></w:pPr><w:rPr><w:color w:val="666666"/><w:sz w:val="16"/><w:szCs w:val="16"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Footer"><w:name w:val="footer"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:pPr><w:spacing w:after="0"/></w:pPr><w:rPr><w:color w:val="666666"/><w:sz w:val="16"/><w:szCs w:val="16"/></w:rPr></w:style>` +
	`<w:style w:type="table" w:styleId="SpecTable"><w:name w:val="Specification Table"/><w:basedOn w:val="TableNormal"/><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:left w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:right w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="A6A6A6"/></w:tblBorders><w:tblCellMar><w:left w:w="85" w:type="dxa"/><w:right w:w="85" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>` +
	`</w:styles>`

// heading is the style of heading level n: size in half points, the space
// before it in twentieths of a point.
func heading(n, size, before int) string {
	return fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="9"/><w:qFormat/><w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="%d" w:after="120"/><w:outlineLvl w:val="%d"/></w:pPr><w:rPr><w:b/><w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr></w:style>`, n, n, before, n-1, size, size)
}

// toc is the style of the table of contents' entries of level n.
func toc(n int) string {
	return fmt.Sprintf(`<w:style w:type="paragraph" w:styleId="TOC%d"><w:name w:val="toc %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="39"/><w:unhideWhenUsed/><w:pPr><w:spacing w:after="40"/><w:ind w:left="%d"/></w:pPr></w:style>`, n, n, 240*(n-1))
}
