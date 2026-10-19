import {test} from 'node:test'
import assert from 'node:assert/strict'
import {qaState,qaQuery,qaParent} from './qaInsightsState.ts'
test('question statistics filters survive detail navigation',()=>{const s=qaState({tab:'questions',page:'3',page_size:'50',source:'wecom',chat_type:'group',start:'2026-10-12',keyword:'license'});assert.deepEqual(qaState({...qaQuery(s),compare:'second',fromTopic:'topic'}),s)})
test('invalid paging and dates get safe defaults',()=>{const s=qaState({page:'-1',page_size:'9999',start:'not-a-date'});assert.equal(s.page,1);assert.equal(s.page_size,20);assert.equal(s.start,'');assert.equal(s.min_count,3);assert.equal(s.min_people,2)})
test('topic detail restores parent filters and page',()=>{const parent=qaQuery(qaState({page:'3',keyword:'数据库',source:'wecom'}));assert.deepEqual(qaParent({page:'1',parent:JSON.stringify(parent)}),parent);assert.equal(qaParent({parent:'bad json'}).page,'1')})
