// Isolated UI fixture, with no production requests or persisted business data.
import {createServer} from 'vite'
const questions=Array.from({length:47},(_,i)=>({id:`q${i}`,question:`服务器数据库同步检查 ${i+1}`,user_id:`u${i%4}`,user_name:`验收用户${i%4+1}`,source:i%2?'web':'wecom',chat_type:i%2?'':'group',classification:'classified',created_at:'2026-10-12T08:30:00Z',topic_id:'t1'}))
const topics=Array.from({length:27},(_,i)=>({id:`t${i}`,title:`服务器数据库同步主题 ${i+1}`,questions:47,people:4,useful:2,not_useful:1,failures:0,last_asked:'2026-10-12T08:30:00Z'}))
questions[0].question='双机单启的服务多起了是什么原因？请结合现场版本、部署模式、错误日志和检查结果说明完整处理步骤。'.repeat(3)
questions[0].user_name=''
questions[0].user_id='woYSFLDQAAVeryLongUserIdentifierForEllipsisReview'
topics[0].title=questions[0].question
const server=await createServer({plugins:[{name:'insights-fixture',configureServer(server){server.middlewares.use(async(req,res,next)=>{
 if(!req.url?.startsWith('/api/v1/'))return next();const url=new URL(req.url,'http://localhost'),p=url.pathname,q=url.searchParams
 let data={data:[]};let raw='';for await(const chunk of req)raw+=chunk
 if(p.endsWith('/memory-status'))data={subjects:4,items:8,pending:1,failed:0}
 else if(p.endsWith('/topics')||p.endsWith('/questions')){const rows=(p.endsWith('/topics')?topics:questions).filter(r=>(r.title||r.question).includes(q.get('keyword')||''));const page=Number(q.get('page')||1),size=Number(q.get('page_size')||20);data={data:rows.slice((page-1)*size,page*size),total:rows.length}}
 else if(p.includes('/questions/')){const question=questions.find(r=>r.id===p.split('/').at(-1));data={data:question,answers:[{id:`a-${question.id}`,answer:'## 检查步骤\n\n先检查数据库运行状态，再检查同步日志。\n\n1. 确认服务可用。\n2. 对照当前版本检查同步配置。',generation_status:'completed',delivery_status:'submitted_unconfirmed',model_name:'验收模型',memory_count:1,duration_ms:4200,references:[{knowledge_title:'数据库同步说明',content:'此处为当次引用片段。'}]}],feedback:[]}}
 else if(p.endsWith('/reclassify'))data={topic_id:'t2',success:true}
 else if(p.includes('/topics/'))data={data:topics.find(t=>t.id===p.split('/').at(-1))||topics[0]}
 res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data))
 })}}],server:{host:'127.0.0.1',port:5189,strictPort:true,open:false}})
await server.listen();console.log('Isolated insights fixture: http://127.0.0.1:5189/tests/feedback/index.html#/platform/qa-insights')
