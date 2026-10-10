package ast

import (
 "encoding/json"
 "testing"
 "go.ziradocs.com/core/v2/diagnostics"
)

func TestListStartSchemaDecoderParity(t *testing.T) {
 schema:=compileASTSchema(t)
 pos:=diagnostics.NewPosition(1,1);doc:=NewAST(pos);b:=NewContentBlock(pos,"content");p:=NewPointsElement(pos);p.ListType="ordered";n:=int64(3);p.Start=&n;p.Items=[]PointItem{*NewPointItem(pos,"One")};b.Elements=[]Element{p};doc.ContentBlocks=[]ContentBlock{*b};SetTableContract(doc)
 raw,_:=json.Marshal(doc)
 var valid map[string]any;_ = json.Unmarshal(raw,&valid)
 if err:=schema.Validate(valid);err!=nil{t.Fatal(err)};if _,err:=DecodeAST(raw);err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{name string;mutate func(map[string]any,map[string]any)}{
  {"zero",func(r,p map[string]any){p["start"]=0}},
  {"unsafe",func(r,p map[string]any){p["start"]=9007199254740992.0}},
  {"fraction",func(r,p map[string]any){p["start"]=1.5}},
  {"null",func(r,p map[string]any){p["start"]=nil}},
  {"string",func(r,p map[string]any){p["start"]="3"}},
  {"unordered",func(r,p map[string]any){p["listType"]="unordered"}},
  {"missing capability",func(r,p map[string]any){delete(r,"capabilities")}},
  {"unused capability",func(r,p map[string]any){delete(p,"start")}},
  {"legacy",func(r,p map[string]any){r["schemaVersion"]=LegacySchemaVersion;delete(r,"capabilities")}},
  {"wrong owner",func(r,p map[string]any){p["subListStart"]=3}},
  {"wrong child type",func(r,p map[string]any){i:=p["items"].([]any)[0].(map[string]any);i["subListType"]="unordered";i["subListStart"]=3;i["subPoints"]=[]any{map[string]any{"type":"point_item","content":"Child","position":map[string]any{"line":1,"column":1},"endPosition":map[string]any{"line":1,"column":1}}};r["capabilities"]=[]any{NestedListTypesCapability,ListStartCapability}}},
 } {t.Run(tc.name,func(t *testing.T){var r map[string]any;_ = json.Unmarshal(raw,&r);p:=r["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any);tc.mutate(r,p);bad,_:=json.Marshal(r);if _,err:=DecodeAST(bad);err==nil{t.Fatal("decoder accepted invalid contract")};if err:=schema.Validate(r);err==nil{t.Fatal("schema accepted invalid contract")}})}
}

func TestMathSourceSchemaDecoderParity(t *testing.T) {
 schema:=compileASTSchema(t);pos:=diagnostics.NewPosition(1,1);doc:=NewAST(pos);b:=NewContentBlock(pos,"content");b.Elements=[]Element{NewMathElement(pos,"  x\n\ny  ")};doc.ContentBlocks=[]ContentBlock{*b};doc.Capabilities=[]string{MathSourceCapability};doc.SchemaVersion=MathSourceSchemaVersion
 raw,_:=json.Marshal(doc);var r map[string]any;_ = json.Unmarshal(raw,&r)
 if _,err:=DecodeAST(raw);err!=nil{t.Fatal(err)};if err:=schema.Validate(r);err!=nil{t.Fatal(err)}
 r["contentBlocks"].([]any)[0].(map[string]any)["elements"]=[]any{};bad,_:=json.Marshal(r)
 if _,err:=DecodeAST(bad);err==nil{t.Fatal("unused math policy accepted")};if err:=schema.Validate(r);err==nil{t.Fatal("schema accepted unused math policy")}
}
