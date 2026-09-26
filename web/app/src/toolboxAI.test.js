import test from "node:test";
import assert from "node:assert/strict";
import { aiCandidatePatch } from "./toolboxAI.js";

test("AI suggestions update only their intended field and enable product copy when adopted", () => {
 const row = { id: "keep-id", target: {id:"keep-image"}, title:"my version", notes:"original", text:"original", claims:"facts", selected:false, adapt:false };
 const notes = {...row,...aiCandidatePatch("model","direction",{text:"自然微笑"})};
 assert.equal(notes.text,row.text); assert.equal(notes.target,row.target); assert.equal(notes.notes,"自然微笑");
 const product = {...row,...aiCandidatePatch("product","product_copy",{text:"商品文案"})};
 assert.equal(product.adapt,true); assert.equal(product.claims,"facts"); assert.equal(product.selected,false);
 const claims = aiCandidatePatch("product","claims",{text:"现有卖点"});
 assert.deepEqual(claims,{claims:"现有卖点"});
 assert.throws(()=>aiCandidatePatch("model","claims",{text:"bad"}));
 assert.throws(()=>aiCandidatePatch("copy","rewrite",{text:" "}));
 assert.equal(row.text,"original");
});
