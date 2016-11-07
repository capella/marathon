import path from 'path'
import Koa from 'koa'
import _ from 'koa-route'
import Logger from '../extensions/logger'
import HealthcheckHandler from './handlers/healthcheck'
import { connect as redisConnect } from '../extensions/redis'
import { connect as pgConnect } from '../extensions/postgresql'
import { connect as kafkaClientConnect } from '../extensions/kafkaClient'
import { connect as kafkaProducerConnect } from '../extensions/kafkaProducer'


export default class MarathonApp {
  constructor(config) {
    this.config = config
    this.allowedMethods = ['get', 'post', 'put', 'delete']
    this.koaApp = new Koa()
    this.configureLogger()
    this.configureMiddleware()

    this.handlersPath = path.join(__dirname, '../api/handlers')
    this.handlers = this.getHandlers()
    this.redisConfig = config.get('app.services.redis')
    this.pgConfig = config.get('app.services.postgresql')
  }

  getHandlers() {
    const self = this
    const handlers = []

    // Include handlers here
    handlers.push(new HealthcheckHandler(self))

    return handlers
  }

  exit(err) {
    if (process.env.NODE_ENV === 'test') {
      throw err
    }
    this.logger.fatal({ err })
    process.exit(1)
  }

  configureLogger() {
    this.logger = new Logger(this.config).logger.child({
      source: 'app',
    })
  }

  async configureRedis() {
    const redisOptions = {
      db: this.redisConfig.db,
      shouldReconnect: this.redisConfig.shouldReconnect,
      password: this.redisConfig.password,
    }
    if (!redisOptions.password) delete redisOptions.password
    try {
      this.redisClient = await redisConnect(
        this.redisConfig.url,
        redisOptions,
        this.logger
      )
    } catch (err) {
      this.exit(err)
    }
  }

  async configurePostgreSQL() {
    try {
      this.db = await pgConnect(
        this.pgConfig.url,
        this.pgConfig.options,
        this.logger
      )
    } catch (err) {
      this.exit(err)
    }
  }

  async configureKafka() {
    try {
      this.logger.debug('Connecting API Kafka client...')
      const cfg = this.config.get('app.services.kafka.api.client')
      this.apiKafkaClient = await kafkaClientConnect(cfg.url, cfg.clientId, this.logger)

      this.logger.debug('Connecting API Kafka producer...')
      const producerCfg = this.config.get('app.services.kafka.api.producer')
      this.apiKafkaProducer = await kafkaProducerConnect(
        this.apiKafkaClient,
        producerCfg,
        this.logger
      )
    } catch (err) {
      this.exit(err)
    }
  }

  async initializeServices() {
    try {
      this.logger.debug('Starting redis configuration...')
      await this.configureRedis()
      this.logger.debug('Starting PostgreSQL configuration...')
      await this.configurePostgreSQL()
      this.logger.debug('Starting Kafka configuration...')
      await this.configureKafka()
    } catch (err) {
      this.exit(err)
    }
  }

  configureMiddleware() {
    this.koaApp.use(async (ctx, next) => {
      const start = new Date()
      await next()
      const ms = new Date() - start
      ctx.set('X-Response-Time', `${ms}ms`)
    })
  }

  async initializeApp() {
    await this.initializeServices()
    this.handlers.forEach((handler) => {
      this.allowedMethods.forEach((methodName) => {
        if (!handler[methodName]) {
          return
        }
        const handlerMethod = handler[methodName].bind(handler)
        const method = _[methodName]
        this.koaApp.use(
          method(handler.route, async (ctx) => {
            await handlerMethod(ctx)
          })
        )
      })
    })
  }

  async run() {
    const PORT = this.config.get('app.port')
    await this.initializeApp()

    this.logger.info(`Listening on port ${PORT}...`)
    this.koaApp.listen(PORT)
  }
}