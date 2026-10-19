export function qaState(query:Record<string,unknown>) {
  const str=(key:string)=> typeof query[key]==='string' ? query[key] as string : ''
  const date=(key:string)=>/^\d{4}-\d{2}-\d{2}$/.test(str(key)) ? str(key) : ''
  return {
    tab:str('tab')==='questions'?'questions':'topics', keyword:str('keyword'),reporter:str('reporter'),source:str('source'),chat_type:str('chat_type'),classification:str('classification'),
    start:date('start'),end:date('end'),frequent:str('frequent')==='false'?'false':'true',
    min_count:Math.max(1,Number(str('min_count'))||3),min_people:Math.max(1,Number(str('min_people'))||2),
    page:Math.floor(Math.min(100000,Math.max(1,Number(str('page'))||1))),page_size:[20,50,100].includes(Number(str('page_size')))?Number(str('page_size')):20,
  }
}
export const qaQuery=(s:ReturnType<typeof qaState>)=>Object.fromEntries(Object.entries(s).map(([k,v])=>[k,String(v)]))
export function qaParent(query:Record<string,unknown>) {
  if(typeof query.parent==='string') { try { return qaQuery(qaState(JSON.parse(query.parent))) } catch {} }
  return qaQuery({...qaState(query),tab:'topics'})
}
