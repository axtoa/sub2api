import type { MonitoredAccount, QualityRun, QualitySettings, QualitySnapshot, QualityTarget } from '../api'

export const settings: QualitySettings = { auto_enabled:true,daily_requests:1000,daily_tokens:2000000,version:7,
  availability_enabled:true,availability_interval_minutes:60,quality_enabled:true,quality_interval_minutes:720,
  cache_enabled:false,cache_interval_minutes:1440,questions:3,pass_percent:80,retry_count:1,retry_interval_minutes:5,max_output_tokens:2048,reasoning_effort:'',cache_prefix_kb:32 }
export const account: MonitoredAccount = { id:1,name:'Account A',platform:'openai',status:'active',enabled:true,version:2,groups:[{id:1,name:'Shared group'},{id:2,name:'Second group'}],unsupported:false }
export const target: QualityTarget = { id:'watch-1',account_id:1,account_name:'Account A',platform:'openai',model:'gpt-model',account_status:'active',enabled:true,account_enabled:true,
  overrides:null,version:3,next_availability_at:'',next_quality_at:'',next_cache_at:'',last_availability_at:null,last_quality_at:null,last_cache_at:null,retry_attempt:0,
  metrics:{requests:0,successes:0,errors:0,first_token_ms:null,billed_input_tokens:0,billed_cache_tokens:0,last_success_at:null},
  recent:[],latest_quality:null,latest_cache:null,pending:null,probe_successes:0,probe_failures:0 }
export const snapshot: QualitySnapshot = { settings,defaults:{...settings,version:1},accounts:[account],targets:[target],budget:{day:'2026-10-23',timezone:'Asia/Shanghai',requests:10,tokens:12345,unconfirmed_requests:1,limited:false,manual_requests:2,manual_tokens:100,manual_unconfirmed:0},estimated_daily_requests:30,server_time:'' }
export const run: QualityRun = { id:'run-1',target_id:target.id,kind:'quality',status:'finished',manual:true,verdict:'suspected',profile_hash:'hash',profile:settings,created_at:'2026-10-23T10:00:00Z',started_at:'2026-10-23T10:00:00Z',finished_at:'2026-10-23T10:00:01Z',
  result:{reason:'',correct:0,graded:1,planned:1,requests:1,budget_tokens:30,first_token_ms:125,duration_ms:1000,cache_reported:false,cache_input_tokens:0,cache_read_tokens:0,items:[{
    question_id:'q1',family:'path_count',prompt:'Count the paths. Return JSON.',expected:'{"answer":8}',explanation:'20 minus 12 equals 8.',correct:false,
    probe:{text:'{"answer":12}',model:'upstream-model',http_status:200,complete:true,reason:'',duration_ms:1000,first_token_ms:125,input_tokens:20,output_tokens:10,cache_read_tokens:0,cache_write_tokens:0,usage_reported:true,cache_reported:false}
  }]} }
