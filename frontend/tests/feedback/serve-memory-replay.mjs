// Isolated browser fixtures: no traffic is forwarded to production.
import {createServer} from 'vite'
let granted=false
const subjects=Array.from({length:23},(_,i)=>({id:`s${i}`,name:i?'测试员工 '+i:'',user_id:'woYSFLDQAAlongEmployeeIdentifier'+i,enabled:!!(i%2),active:2,pending:1,queued:0,failed:i%3===0?1:0,last_extracted_at:'2026-10-13T09:00:00Z'}))
const proposal={id:'p1',feedback_id:'f1',question:'数据库同步失败怎么办？',candidate_answer:'先检查连接，再检查同步日志。',knowledge_base_id:'kb',status:'draft',conflict_status:'none',regression_run_id:''}
let runs=[],results=[]
const server=await createServer({plugins:[{name:'memory-replay-fixture',configureServer(server){server.middlewares.use(async(req,res,next)=>{
 if(!req.url?.startsWith('/api/v1/'))return next()
 const u=new URL(req.url,'http://localhost'),p=u.pathname;let raw='';for await(const b of req)raw+=b;const body=raw?JSON.parse(raw):{};let data={data:[]}
 if(p.endsWith('/memory/access')){if(req.method==='PUT')granted=body.enabled;data={data:{granted,owner:true,members:[{user_id:'test-admin',name:'测试管理员',role:'owner',granted}]}}}
 else if(p.endsWith('/memory/subjects')){const page=Number(u.searchParams.get('page')||1),rows=subjects.filter(s=>(s.name+s.user_id).includes(u.searchParams.get('keyword')||''));data={data:rows.slice((page-1)*20,page*20),total:rows.length}}
 else if(p.includes('/memory/subjects/')){if(p.endsWith('/content'))data={data:[{id:'item',kind:'preference',status:'active',origin:'explicit',content:'回答先给结论，再给步骤。',updated_at:'2026-10-13T09:00:00Z'}],total:1};else data={data:{enabled:true,last_extracted_at:'2026-10-13T09:00:00Z',jobs:[{session_id:'session-a',pending:false,failure_count:0,failure_code:'',updated_at:'2026-10-13T09:00:00Z'},{session_id:'session-b',pending:true,failure_count:2,failure_code:'invalid_output',updated_at:'2026-10-13T08:00:00Z'}]}}}
 else if(p.endsWith('/agents'))data={data:[{id:'builtin-quick-answer',name:'快速问答',config:{agent_mode:'quick-answer'}}]}
 else if(p.includes('/answer-feedback/'))data={data:{id:'f1',user_name:'测试用户',user_id:'alice',source:'wecom',status:proposal.status==='published'?'resolved':'correcting',rating:'not_useful',question:proposal.question,answer:'原回答需要补充',reason:'请补充',created_at:'2026-10-13T09:00:00Z'},proposal,regression:runs.find(r=>r.id===proposal.regression_run_id)||null,references:[]}
 else if(p.endsWith('/runs'))data={data:runs,results,proposal}
 else if(p.endsWith('/regression')||p.endsWith('/verify')){const live=p.endsWith('/verify'),run={id:`r${runs.length}`,kind:live?'published':'preview',status:'passed',passed:1,total:1,input:body,agent_id:'builtin-quick-answer',model_id:'验收模型',started_at:'2026-10-13T09:00:00Z'};runs.unshift(run);results.push({id:run.id,run_id:run.id,question:proposal.question,answer:'## 处理步骤\n\n先检查连接，再检查同步日志。',status:'passed',evaluation:'关键结论与标准答案一致',duration_ms:3500,references:[{knowledge_title:live?'已发布纠错':'候选纠错（未发布）',content:body.answer}]});if(!live){proposal.regression_run_id=run.id;proposal.candidate_answer=body.answer;proposal.status='ready'}data={data:run,proposal}}
 else if(p.endsWith('/publish')){proposal.status='published';proposal.correction_knowledge_id='new';data={data:proposal,knowledge_id:'new'}}
 else if(p.includes('knowledge-bases'))data={data:[{id:'kb',name:'XBROTHER'}]}
 res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data))
 })}}],server:{host:'127.0.0.1',port:5190,strictPort:true,open:false}})
await server.listen();console.log('UI fixture: http://127.0.0.1:5190/tests/feedback/index.html#/platform/qa-insights/memory')
